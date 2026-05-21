package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Параметры Argon2id (OWASP 2023 для интерактивного логина):
//   memory  = 64 MiB
//   time    = 3
//   threads = 2
//   keyLen  = 32 байта (256 бит)
//   saltLen = 16 байт (128 бит)
// На современном CPU это ~150-250ms, что:
//  • невозможно перебрать на GPU-ферме
//  • достаточно быстро, чтобы не лагать (всего 1 раз на login/register, без блокировки роутера)
const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // KiB → 64 MiB
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

var (
	ErrInvalidHash  = errors.New("invalid hash format")
	ErrIncompatible = errors.New("incompatible argon2 hash")
)

// HashPassword возвращает PHC-строку:
//   $argon2id$v=19$m=65536,t=3,p=2$<salt-b64>$<hash-b64>
// Эту строку и кладём в users.password_hash. Никогда не храним сам пароль.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword: парсит PHC-строку, пересчитывает Argon2id с теми же параметрами
// и сравнивает в постоянное время (subtle.ConstantTimeCompare) — защита от timing-атак.
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	if version != argon2.Version {
		return false, ErrIncompatible
	}
	var memory, t uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &t, &threads); err != nil {
		return false, ErrInvalidHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	wantHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}
	keyLen := uint32(len(wantHash))
	gotHash := argon2.IDKey([]byte(password), salt, t, memory, threads, keyLen)
	return subtle.ConstantTimeCompare(gotHash, wantHash) == 1, nil
}
