# KirpichMusic

Самохостящийся музыкальный сервис: бэкенд на Go + PostgreSQL, фронт — vanilla JS/HTML/CSS (один файл, без сборки).

## Стек

- Go 1.22 + chi router (минимализм поверх stdlib `net/http`)
- PostgreSQL 14+ — `citext`, `pg_trgm` (full-text), `pgcrypto`
- pgx/v5 — пул соединений + параметризованные запросы
- Argon2id (`golang.org/x/crypto/argon2`) — хеширование паролей
- Сессии: HttpOnly + Secure (prod) + SameSite=Strict cookie; в БД лежит только SHA-256(token)
- ffmpeg (внешний) — генерация waveform peaks при upload
- Pure Go — собирается без cgo. Кроссплатформенно: Linux/OpenBSD/macOS без правок.

## Структура

```
cmd/server/main.go              — точка входа, маршруты chi, graceful shutdown
internal/config/                — env-конфиг + .env loader (без зависимостей)
internal/db/                    — pgxpool
internal/httpx/                 — JSON-helpers, security headers, типизированные ошибки
internal/auth/                  — Register / Login / Logout / Me / UpdateMe / DeleteMe + middleware
internal/tracks/                — upload, edit, delete, отдача аудио/обложек, waveform
internal/comments/              — комментарии к трекам
internal/follows/               — подписки, top uploaders (materialized view)
internal/playlists/             — плейлисты + tracks
internal/storage/               — файловое хранилище (atomic rename, MIME-sniff)
db/migrations/
    001_init.sql                — users, tracks, comments, likes, sessions
    002_follows_and_waveform.sql — follows, top_uploaders MV, waveform_peaks
    003_users_and_playlists.sql  — bio, avatar, playlists, playlist_tracks
    004_admin_and_pageinfo.sql   — is_admin + auto-promote первого юзера
    005_avatars_and_banners.sql  — avatar_path/avatar_mime/avatar_bytes,
                                    banner_path/banner_mime/banner_bytes
    006_artists.sql              — artists (CITEXT name UNIQUE, slug,
                                    avatar/banner, claimed_by_user_id,
                                    is_verified) + tracks.artist_id FK
    007_moderation.sql           — artist_avatar_proposals + tracks.moderation_*
internal/admin/                 — /api/admin/uptime (RAM, диск, БД, аптайм),
                                  управление пользователями + moderation очереди
internal/artists/               — CRUD артистов, /suggest fuzzy (pg_trgm),
                                  avatar/banner upload, proposals,
                                  admin claim к user-аккаунту
internal/users/                 — публичный профиль /api/users/by-username,
                                  upload/serve/delete avatar+banner,
                                  followers/following списки
setup.sh, run.sh                — полная установка / простой старт
web/index.html                  — фронт целиком в одном файле
```

## Эндпоинты

### Auth
| Метод | Путь | Описание | Auth |
|---|---|---|---|
| POST   | `/api/auth/register` | `{username,email,password}` → 201 + cookie | – |
| POST   | `/api/auth/login`    | `{login,password}` (login = email или username) | – |
| POST   | `/api/auth/logout`   | revokes session, clears cookie | – |
| GET    | `/api/auth/me`       | расширенный профиль со счётчиками | cookie |
| PATCH  | `/api/auth/me`       | смена username/email/bio/пароля | cookie |
| DELETE | `/api/auth/me`       | удалить аккаунт + опционально треки | cookie |

### Tracks
| Метод | Путь | Описание | Auth |
|---|---|---|---|
| GET    | `/api/tracks/`              | `?q=` поиск, `?sort=recent\|popular`, `?genre=` | optional |
| GET    | `/api/tracks/{id}`          | детальный трек + `liked_by_me` | optional |
| GET    | `/api/tracks/{id}/audio`    | стрим, поддержка HTTP Range | – |
| GET    | `/api/tracks/{id}/cover`    | обложка | – |
| POST   | `/api/tracks/`              | multipart upload | cookie |
| PATCH  | `/api/tracks/{id}`          | редактирование (только владелец) | cookie |
| DELETE | `/api/tracks/{id}`          | удаление (только владелец) | cookie |
| POST   | `/api/tracks/{id}/like`     | лайк | cookie |
| DELETE | `/api/tracks/{id}/like`     | анлайк | cookie |
| GET    | `/api/tracks/{id}/comments` | список комментариев | – |
| POST   | `/api/tracks/{id}/comments` | оставить комментарий | cookie |

### Users
| Метод | Путь | Описание | Auth |
|---|---|---|---|
| GET    | `/api/users/top`            | топ-N самых прослушиваемых | – |
| POST   | `/api/users/{id}/follow`    | подписка | cookie |
| DELETE | `/api/users/{id}/follow`    | отписка | cookie |

### Playlists
| Метод | Путь | Описание | Auth |
|---|---|---|---|
| GET    | `/api/playlists/?user_id=`              | список плейлистов | optional |
| GET    | `/api/playlists/{id}`                   | плейлист + треки | optional |
| POST   | `/api/playlists/`                       | создать | cookie |
| PATCH  | `/api/playlists/{id}`                   | переименовать/изменить | cookie |
| DELETE | `/api/playlists/{id}`                   | удалить | cookie |
| POST   | `/api/playlists/{id}/tracks`            | добавить трек | cookie |
| DELETE | `/api/playlists/{id}/tracks/{trackId}`  | удалить трек | cookie |

## Запуск (dev)

### Самое простое — `setup.sh`

```bash
./setup.sh   # детектит дистрибутив, ставит Go + PostgreSQL,
             # создаёт роль/БД, накатывает все миграции, собирает
             # бинарь и стартует сервер.
./run.sh     # повторный запуск (без переустановок)
```

`setup.sh` поддерживает: apt (Debian/Ubuntu), dnf/yum (Fedora/RHEL),
pacman (Arch/Manjaro), zypper (openSUSE), apk (Alpine), brew (macOS).

Параметры можно переопределить переменными окружения:
`DB_NAME`, `DB_USER`, `DB_PASS`, `DB_HOST`, `DB_PORT`, `HTTP_ADDR`.
`NO_RUN=1 ./setup.sh` — не стартовать сервер по завершении.

### Вручную

```bash
sudo -u postgres psql <<'SQL'
CREATE USER kirpich WITH PASSWORD 'kirpich';
CREATE DATABASE kirpichmusic OWNER kirpich;
\connect kirpichmusic
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
SQL

for m in db/migrations/*.sql; do
  PGPASSWORD=kirpich psql -h 127.0.0.1 -U kirpich -d kirpichmusic -f "$m"
done

cp .env.example .env
go build -trimpath -o bin/kirpichmusic ./cmd/server
./bin/kirpichmusic
# открыть http://localhost:8080
```

## Безопасность

- **Пароли** — Argon2id (m=64MiB, t=3, p=2). Никогда в открытом виде.
- **SQL-инъекции** — все запросы параметризованы (`$1, $2, ...`).
- **Сессии** — `HttpOnly` + `Secure` (prod) + `SameSite=Strict`. Токен в БД хранится как `SHA-256`.
- **Timing-атаки** — `subtle.ConstantTimeCompare` при проверке пароля.
- **JSON-DoS** — `MaxBytesReader` 1 MiB на body + `DisallowUnknownFields`.
- **XSS на фронте** — все user-generated строки экранируются (`escapeHtml`/`escapeAttr`).
- **CSRF** — SameSite=Strict + проверка cookie+уникального токена.
- **Multipart upload** — MIME через magic bytes (`gabriel-vasile/mimetype`), не доверяем `Content-Type`.

## Возможности фронтенда

- Регистрация / вход / выход
- Загрузка треков (multipart, drag-and-drop, превью обложки)
- Редактирование треков (название, артист, жанр, обложка)
- Удаление треков
- Плейлисты: создание / редактирование / удаление, добавление и удаление треков
- Лайки + комментарии (через бэк)
- Подписки + Recommended sidebar (топ-3 по сумме `plays_count`)
- Поиск через бэк (`pg_trgm`, debounced)
- Личный кабинет: смена username/email/bio/пароля, удаление аккаунта
- Аватар + шапка профиля (transactional upload, MIME-sniff)
- Подписки/Подписчики: списки `/user/{id}/followers`, `/user/{id}/following`
- Публичный профиль `/user/{username}`
- **Артисты** — отдельная сущность с `/artist/<slug>`: баннер, аватар, описание, треки
- **Анти-дубль** при загрузке: fuzzy autocomplete по pg_trgm + точные совпадения через CITEXT
- **Народная модерация**: предложение аватарки артиста любым залогиненным юзером
- **Верификация**: админ привязывает артиста к user-аккаунту; от не-владельцев треки уходят в очередь модерации
- **Админ-панель** /admin: 5 вкладок (Состояние / Заявки / Треки / Артисты / Пользователи) — одобрение аватарок, модерация треков, управление ролями (`is_admin`), claim артистов
- Now-playing: equalizer-индикатор + переключение play→pause при наведении
- **Режим производительности** (3 ступени): Полный / Сбалансированный / Быстрый — отключают `backdrop-filter`, тяжёлые градиенты и анимации (CSS-классы на `<html>`, persistent в localStorage)
- Плавные transitions (180–220ms ease-out)

## Артисты, модерация, верификация

- При загрузке трека артист берётся:
  1. По `artist_id` (если выбран в autocomplete) — ничего не дублируется,
  2. Иначе по `CITEXT`-совпадению (case-insensitive) с `artists.name`,
  3. Иначе создаётся новая запись с автоматическим `slug` (translit RU→LAT).
- `GET /api/artists/suggest?q=...&limit=5` — fuzzy через `similarity()` (pg_trgm), порог 0.2.
- Если у артиста стоит `claimed_by_user_id` (выставляется админом через `POST /api/admin/artists/{id}/claim`), то трек, загруженный другим юзером, попадает в очередь `tracks.moderation_status='pending'`. Админ одобряет/отклоняет через `/api/admin/tracks/{id}/approve|reject`.
- Любой залогиненный юзер может прислать аватарку: `POST /api/artists/{id}/proposals` (multipart). Очередь видна админу в `/api/admin/artists/proposals`. Approve копирует картинку в основной аватар артиста.
- Назначить/снять админа: `PATCH /api/admin/users/{id}` `{is_admin: true|false}`. Самому себе снять нельзя.

### Эндпоинты Artists / Admin (v7)

| Метод | Путь | Описание | Auth |
|---|---|---|---|
| GET    | `/api/artists/`                         | список артистов | optional |
| GET    | `/api/artists/suggest?q=`               | fuzzy autocomplete | optional |
| GET    | `/api/artists/by-slug/{slug}`           | публичный профиль артиста | optional |
| GET    | `/api/artists/{id}`                     | то же по id | optional |
| GET    | `/api/artists/{id}/avatar`              | аватар (стрим) | – |
| GET    | `/api/artists/{id}/banner`              | шапка | – |
| GET    | `/api/artists/proposals/{pid}/image`    | картинка предложения | – |
| PATCH  | `/api/artists/{id}`                     | description (claimed_by или admin) | cookie |
| POST   | `/api/artists/{id}/avatar`              | загрузить аватар (claimed_by или admin) | cookie |
| DELETE | `/api/artists/{id}/avatar`              | удалить аватар | cookie |
| POST   | `/api/artists/{id}/banner`              | загрузить шапку | cookie |
| DELETE | `/api/artists/{id}/banner`              | удалить шапку | cookie |
| POST   | `/api/artists/{id}/proposals`           | предложить аватар (любой юзер) | cookie |
| GET    | `/api/admin/uptime`                     | uptime/CPU/RAM/диск/БД | admin |
| GET    | `/api/admin/artists/proposals?status=`  | очередь предложений | admin |
| POST   | `/api/admin/artists/proposals/{pid}/approve` | одобрить (копирует в аватар) | admin |
| POST   | `/api/admin/artists/proposals/{pid}/reject`  | отклонить | admin |
| POST   | `/api/admin/artists/{id}/claim`         | привязать артиста к user'у | admin |
| GET    | `/api/admin/users?q=&limit=`            | список юзеров | admin |
| PATCH  | `/api/admin/users/{id}`                 | `{is_admin, is_active}` | admin |
| GET    | `/api/admin/tracks/pending`             | очередь треков на модерации | admin |
| POST   | `/api/admin/tracks/{id}/approve`        | опубликовать | admin |
| POST   | `/api/admin/tracks/{id}/reject`         | удалить файл + запись | admin |
| DELETE | `/api/admin/tracks/{id}`                | хард-делит любого трека | admin |

## Кроссплатформенная сборка

```bash
GOOS=linux   GOARCH=amd64 go build -trimpath -o bin/kirpichmusic-linux   ./cmd/server
GOOS=openbsd GOARCH=amd64 go build -trimpath -o bin/kirpichmusic-openbsd ./cmd/server
GOOS=darwin  GOARCH=arm64 go build -trimpath -o bin/kirpichmusic-mac     ./cmd/server
```
