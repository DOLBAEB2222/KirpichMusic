package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bogdan/kirpichmusic/internal/httpx"
)

type Handler struct {
	pool     *pgxpool.Pool
	sessions *SessionStore
}

func NewHandler(pool *pgxpool.Pool, sessions *SessionStore) *Handler {
	return &Handler{pool: pool, sessions: sessions}
}

type registerReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginReq struct {
	// Login — email или username (точно как в HTML-прототипе: один input).
	Login    string `json:"login"`
	Password string `json:"password"`
}

type userDTO struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// userMeDTO — расширенный профиль текущего пользователя со счётчиками.
type userMeDTO struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	Bio            string `json:"bio"`
	IsAdmin        bool   `json:"is_admin"`
	HasAvatar      bool   `json:"has_avatar"`
	HasBanner      bool   `json:"has_banner"`
	TracksCount    int64  `json:"tracks_count"`
	FollowersCount int64  `json:"followers_count"`
	FollowingCount int64  `json:"following_count"`
	LikesCount     int64  `json:"likes_count"`
	CreatedAt      string `json:"created_at"`
}

// Register — POST /api/auth/register
//   1) валидация (длина username, формат email, минимальная длина пароля)
//   2) Argon2id-хеширование (CPU-heavy — НЕ блокирует другие соединения, т.к. мы в горутине handler'а)
//   3) INSERT через параметризованный запрос ($1, $2, $3) — SQL-injection невозможна
//   4) старт сессии + Set-Cookie
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteBadRequest(w, "Некорректный JSON")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if !validUsername(req.Username) {
		httpx.WriteBadRequest(w, "Username должен быть 2..32 символа (буквы, цифры, _ . -)")
		return
	}
	if !validEmail(req.Email) {
		httpx.WriteBadRequest(w, "Некорректный email")
		return
	}
	if len(req.Password) < 8 {
		httpx.WriteBadRequest(w, "Пароль минимум 8 символов")
		return
	}
	if len(req.Password) > 1024 {
		httpx.WriteBadRequest(w, "Пароль слишком длинный")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		httpx.WriteInternal(w)
		return
	}

	var u userDTO
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, username, email
	`, req.Username, req.Email, hash).Scan(&u.ID, &u.Username, &u.Email)

	if err != nil {
		if isUniqueViolation(err) {
			httpx.WriteError(w, http.StatusConflict, "Email или username уже заняты", "conflict")
			return
		}
		if isCheckViolation(err) {
			httpx.WriteBadRequest(w, "Не прошла серверная валидация")
			return
		}
		httpx.WriteInternal(w)
		return
	}

	if err := h.startSession(w, r, u.ID); err != nil {
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"user": u})
}

// Login — POST /api/auth/login
// Принимает {login, password}, где login — email ИЛИ username (как в doLogin).
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteBadRequest(w, "Некорректный JSON")
		return
	}
	req.Login = strings.TrimSpace(req.Login)
	if req.Login == "" || req.Password == "" {
		httpx.WriteBadRequest(w, "login и password обязательны")
		return
	}

	var (
		u    userDTO
		hash string
	)
	// Параметризованный запрос. CITEXT-сравнение → регистронезависимо для email/username.
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, username, email, password_hash FROM users
		WHERE (username = $1 OR email = $1) AND is_active = TRUE
	`, req.Login).Scan(&u.ID, &u.Username, &u.Email, &hash)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Намеренно общая формулировка — не выдаём, что email существует.
			httpx.WriteError(w, http.StatusUnauthorized, "Неверный логин или пароль", "invalid_credentials")
			return
		}
		httpx.WriteInternal(w)
		return
	}

	ok, err := VerifyPassword(req.Password, hash)
	if err != nil || !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Неверный логин или пароль", "invalid_credentials")
		return
	}

	if err := h.startSession(w, r, u.ID); err != nil {
		httpx.WriteInternal(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": u})
}

// Logout — POST /api/auth/logout
// Идемпотентен: и без куки, и с истёкшей сессией просто отвечает 200.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	tok, err := h.sessions.Cookie(r)
	if err == nil && tok != "" {
		_ = h.sessions.Revoke(r.Context(), tok)
	}
	h.sessions.ClearCookie(w)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Me — GET /api/auth/me (под RequireAuth middleware).
// Возвращает расширенный профиль со счётчиками: треки, подписчики,
// подписки, лайки. Все агрегаты считаются в одном запросе.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	uid := UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	var (
		u         userMeDTO
		bio       *string
		createdAt time.Time
	)
	err := h.pool.QueryRow(r.Context(), `
		SELECT
			u.id, u.username, u.email, u.bio, u.is_admin,
			(u.avatar_path IS NOT NULL),
			(u.banner_path IS NOT NULL),
			u.created_at,
			(SELECT COUNT(*) FROM tracks   t WHERE t.uploader_id = u.id AND t.is_published = TRUE)::BIGINT,
			(SELECT COUNT(*) FROM follows  f WHERE f.followee_id = u.id)::BIGINT,
			(SELECT COUNT(*) FROM follows  f WHERE f.follower_id = u.id)::BIGINT,
			(SELECT COUNT(*) FROM likes    l WHERE l.user_id     = u.id)::BIGINT
		FROM users u
		WHERE u.id = $1 AND u.is_active = TRUE
	`, uid).Scan(&u.ID, &u.Username, &u.Email, &bio, &u.IsAdmin,
		&u.HasAvatar, &u.HasBanner,
		&createdAt,
		&u.TracksCount, &u.FollowersCount, &u.FollowingCount, &u.LikesCount)
	if err != nil {
		httpx.WriteUnauthorized(w)
		return
	}
	u.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	if bio != nil {
		u.Bio = *bio
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": u})
}

// updateMeReq — частичный апдейт профиля. Все поля опциональны:
// присылается только то, что меняется.
type updateMeReq struct {
	Username        *string `json:"username,omitempty"`
	Email           *string `json:"email,omitempty"`
	Bio             *string `json:"bio,omitempty"`
	NewPassword     *string `json:"new_password,omitempty"`
	CurrentPassword *string `json:"current_password,omitempty"`
}

// UpdateMe — PATCH /api/auth/me
// Обновляет username/email/bio/пароль текущего юзера.
// При смене пароля или email/username требуется current_password.
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	uid := UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	var req updateMeReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteBadRequest(w, "Некорректный JSON")
		return
	}

	// Защита: смена email/username/пароля требует current_password.
	requirePwd := req.NewPassword != nil ||
		(req.Email != nil) || (req.Username != nil)
	if requirePwd {
		if req.CurrentPassword == nil || *req.CurrentPassword == "" {
			httpx.WriteBadRequest(w, "Введите текущий пароль")
			return
		}
		var hash string
		err := h.pool.QueryRow(r.Context(),
			`SELECT password_hash FROM users WHERE id=$1 AND is_active=TRUE`, uid,
		).Scan(&hash)
		if err != nil {
			httpx.WriteUnauthorized(w)
			return
		}
		ok, err := VerifyPassword(*req.CurrentPassword, hash)
		if err != nil || !ok {
			httpx.WriteError(w, http.StatusForbidden, "Неверный текущий пароль", "wrong_password")
			return
		}
	}

	// Собираем динамический UPDATE.
	sets := []string{}
	args := []any{}
	argIdx := 1
	add := func(col string, val any) {
		sets = append(sets, col+" = $"+strconv.Itoa(argIdx))
		args = append(args, val)
		argIdx++
	}

	if req.Username != nil {
		uname := strings.TrimSpace(*req.Username)
		if !validUsername(uname) {
			httpx.WriteBadRequest(w, "Username должен быть 2..32 символа (буквы, цифры, _ . -)")
			return
		}
		add("username", uname)
	}
	if req.Email != nil {
		em := strings.TrimSpace(strings.ToLower(*req.Email))
		if !validEmail(em) {
			httpx.WriteBadRequest(w, "Некорректный email")
			return
		}
		add("email", em)
	}
	if req.Bio != nil {
		bio := strings.TrimSpace(*req.Bio)
		if len(bio) > 500 {
			httpx.WriteBadRequest(w, "Описание не больше 500 символов")
			return
		}
		// Пустая строка → NULL в БД, чтобы не плодить пустые поля.
		if bio == "" {
			sets = append(sets, "bio = NULL")
		} else {
			add("bio", bio)
		}
	}
	if req.NewPassword != nil {
		if len(*req.NewPassword) < 8 {
			httpx.WriteBadRequest(w, "Пароль минимум 8 символов")
			return
		}
		if len(*req.NewPassword) > 1024 {
			httpx.WriteBadRequest(w, "Пароль слишком длинный")
			return
		}
		newHash, err := HashPassword(*req.NewPassword)
		if err != nil {
			httpx.WriteInternal(w)
			return
		}
		add("password_hash", newHash)
	}

	if len(sets) == 0 {
		httpx.WriteBadRequest(w, "Нечего обновлять")
		return
	}

	args = append(args, uid)
	q := "UPDATE users SET " + strings.Join(sets, ", ") + " WHERE id = $" + strconv.Itoa(argIdx) + " AND is_active = TRUE"
	if _, err := h.pool.Exec(r.Context(), q, args...); err != nil {
		if isUniqueViolation(err) {
			httpx.WriteError(w, http.StatusConflict, "Email или username уже заняты", "conflict")
			return
		}
		if isCheckViolation(err) {
			httpx.WriteBadRequest(w, "Не прошла серверная валидация")
			return
		}
		httpx.WriteInternal(w)
		return
	}

	// Возвращаем обновлённый профиль.
	h.Me(w, r)
}

// deleteMeReq — удаление аккаунта.
//   delete_tracks=true → перед удалением юзера явно удаляются его треки
//                       (и связанные file-blob'ы, плейлисты, комменты).
//   delete_tracks=false → треки остаются с uploader_id=NULL ("Удалённый аккаунт").
type deleteMeReq struct {
	CurrentPassword string `json:"current_password"`
	DeleteTracks    bool   `json:"delete_tracks"`
}

// DeleteMe — DELETE /api/auth/me
// Требует current_password. Идёт в транзакции.
func (h *Handler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	uid := UserIDFromContext(r.Context())
	if uid == 0 {
		httpx.WriteUnauthorized(w)
		return
	}
	var req deleteMeReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteBadRequest(w, "Некорректный JSON")
		return
	}
	if req.CurrentPassword == "" {
		httpx.WriteBadRequest(w, "Введите текущий пароль")
		return
	}
	var hash string
	err := h.pool.QueryRow(r.Context(),
		`SELECT password_hash FROM users WHERE id=$1 AND is_active=TRUE`, uid,
	).Scan(&hash)
	if err != nil {
		httpx.WriteUnauthorized(w)
		return
	}
	ok, err := VerifyPassword(req.CurrentPassword, hash)
	if err != nil || !ok {
		httpx.WriteError(w, http.StatusForbidden, "Неверный пароль", "wrong_password")
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.WriteInternal(w)
		return
	}
	defer tx.Rollback(r.Context())

	if req.DeleteTracks {
		// Файлы на диске удаляем оппортунистически в фоне:
		// тут только удаляем строки. uploader_id у треков ON DELETE SET NULL,
		// поэтому без этого DELETE треки бы остались.
		if _, err := tx.Exec(r.Context(),
			`DELETE FROM tracks WHERE uploader_id = $1`, uid); err != nil {
			httpx.WriteInternal(w)
			return
		}
	}
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM users WHERE id = $1`, uid); err != nil {
		httpx.WriteInternal(w)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteInternal(w)
		return
	}

	h.sessions.ClearCookie(w)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token, expires, err := h.sessions.Create(r.Context(), userID, r.UserAgent(), clientIP(r))
	if err != nil {
		return err
	}
	h.sessions.SetCookie(w, token, expires)
	return nil
}
