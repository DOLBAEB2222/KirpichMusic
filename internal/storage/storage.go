package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
)

// Storage инкапсулирует операции с файлами на диске.
// Все пути валидируются на path traversal, MIME определяется по magic bytes.
type Storage struct {
	root     string // абсолютный путь к корню медиа-хранилища
	maxBytes int64
}

func New(root string, maxBytes int64) (*Storage, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir media root: %w", err)
	}
	return &Storage{root: abs, maxBytes: maxBytes}, nil
}

// Root возвращает корень для проверок path-traversal.
func (s *Storage) Root() string { return s.root }

var (
	// AllowedAudioMIMEs — то же что в DB CHECK constraint
	AllowedAudioMIMEs = map[string]string{
		"audio/mpeg":  ".mp3",
		"audio/wave":  ".wav",
		"audio/x-wav": ".wav",
		"audio/wav":   ".wav",
		"audio/flac":  ".flac",
		"audio/x-flac": ".flac",
		"audio/aac":   ".aac",
		"audio/mp4":   ".m4a",
		"audio/x-m4a": ".m4a",
		"audio/ogg":   ".ogg",
	}
	// AllowedCoverMIMEs — JPG/PNG/WEBP. SVG намеренно НЕ разрешён (XSS вектор).
	AllowedCoverMIMEs = map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
	}

	ErrUnsupportedMIME = errors.New("unsupported file type")
	ErrTooLarge        = errors.New("file too large")
	ErrEmpty           = errors.New("file is empty")
)

// Stored — итог записи файла.
type Stored struct {
	RelPath string // относительный путь от root, для записи в БД (audio_path / cover_path)
	AbsPath string // абсолютный путь
	MIME    string // нормализованный MIME (например audio/mpeg)
	Bytes   int64
}

// SaveAudio: стримит аудио из multipart-загрузки, проверяет MIME по magic bytes,
// и атомарно перемещает в финальное место. Имя файла — случайное (32 hex chars).
// Возвращает Stored или ошибку.
//
// Безопасность:
//   • size limit до записи (http.MaxBytesReader на уровне handler'а + ограничение здесь)
//   • MIME из первых 3072 байт (mimetype.Detect), а не из расширения/заголовка клиента
//   • файл сначала пишется во временный путь, потом os.Rename → атомарность
//   • случайное имя, никогда не используем filename из клиента (защита от
//     traversal/перезаписи и от рекогносцировки имён)
func (s *Storage) SaveAudio(file multipart.File, header *multipart.FileHeader) (*Stored, error) {
	return s.saveFile(file, header, "audio", AllowedAudioMIMEs)
}

func (s *Storage) SaveCover(file multipart.File, header *multipart.FileHeader) (*Stored, error) {
	return s.saveFile(file, header, "covers", AllowedCoverMIMEs)
}

func (s *Storage) SaveAvatar(file multipart.File, header *multipart.FileHeader) (*Stored, error) {
	return s.saveFile(file, header, "avatars", AllowedCoverMIMEs)
}

func (s *Storage) SaveBanner(file multipart.File, header *multipart.FileHeader) (*Stored, error) {
	return s.saveFile(file, header, "banners", AllowedCoverMIMEs)
}

// SaveArtistAvatar — аватарка артиста (отдельный неймспейс, чтобы не путать с user-avatars).
func (s *Storage) SaveArtistAvatar(file multipart.File, header *multipart.FileHeader) (*Stored, error) {
	return s.saveFile(file, header, "artists/avatars", AllowedCoverMIMEs)
}

// SaveArtistBanner — шапка артиста.
func (s *Storage) SaveArtistBanner(file multipart.File, header *multipart.FileHeader) (*Stored, error) {
	return s.saveFile(file, header, "artists/banners", AllowedCoverMIMEs)
}

// SaveArtistProposal — народное предложение аватара (на модерации).
func (s *Storage) SaveArtistProposal(file multipart.File, header *multipart.FileHeader) (*Stored, error) {
	return s.saveFile(file, header, "artists/proposals", AllowedCoverMIMEs)
}

func (s *Storage) saveFile(file multipart.File, header *multipart.FileHeader, kind string, allowed map[string]string) (*Stored, error) {
	if header.Size <= 0 {
		return nil, ErrEmpty
	}
	if s.maxBytes > 0 && header.Size > s.maxBytes {
		return nil, ErrTooLarge
	}

	// 1) MIME по magic bytes (первые 3072 байта).
	mtype, err := mimetype.DetectReader(file)
	if err != nil {
		return nil, fmt.Errorf("detect mime: %w", err)
	}
	mimeStr := mtype.String()
	// mimetype может возвращать "audio/mpeg; charset=binary" — отбрасываем параметры.
	if i := strings.Index(mimeStr, ";"); i >= 0 {
		mimeStr = strings.TrimSpace(mimeStr[:i])
	}
	ext, ok := allowed[mimeStr]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedMIME, mimeStr)
	}

	// 2) Сбрасываем reader на начало (DetectReader его прочитал).
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	// 3) Каталог по дате — равномерный listdir даже при миллионах файлов.
	now := time.Now().UTC()
	relDir := filepath.Join(kind, now.Format("2006"), now.Format("01"))
	absDir := filepath.Join(s.root, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return nil, err
	}

	// 4) Случайное имя файла (16 байт = 32 hex chars).
	name, err := randomName()
	if err != nil {
		return nil, err
	}
	finalName := name + ext
	relPath := filepath.Join(relDir, finalName)
	absPath := filepath.Join(absDir, finalName)
	tmpPath := absPath + ".tmp"

	// 5) Стримим через io.LimitReader (двойной ремень помимо MaxBytesReader выше).
	var lr io.Reader = file
	if s.maxBytes > 0 {
		lr = io.LimitReader(file, s.maxBytes+1)
	}

	out, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	written, copyErr := io.Copy(out, lr)
	if cerr := out.Close(); cerr != nil && copyErr == nil {
		copyErr = cerr
	}
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return nil, copyErr
	}
	if s.maxBytes > 0 && written > s.maxBytes {
		_ = os.Remove(tmpPath)
		return nil, ErrTooLarge
	}

	// 6) Атомарное "промисное" переименование.
	if err := os.Rename(tmpPath, absPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	return &Stored{
		RelPath: filepath.ToSlash(relPath), // в БД храним POSIX-стиль
		AbsPath: absPath,
		MIME:    mimeStr,
		Bytes:   written,
	}, nil
}

// SafePath проверяет, что относительный путь действительно лежит внутри корня (защита
// от path-traversal, например "../../etc/passwd"). Возвращает абсолютный путь.
func (s *Storage) SafePath(rel string) (string, error) {
	cleaned := filepath.Clean("/" + rel) // принудительное "/" гарантирует absoluteness
	abs := filepath.Join(s.root, cleaned)
	abs = filepath.Clean(abs)
	if !strings.HasPrefix(abs, s.root+string(filepath.Separator)) && abs != s.root {
		return "", errors.New("invalid media path")
	}
	return abs, nil
}

// Remove — удалить файл по относительному пути (с traversal-защитой).
func (s *Storage) Remove(rel string) error {
	abs, err := s.SafePath(rel)
	if err != nil {
		return err
	}
	return os.Remove(abs)
}

func randomName() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
