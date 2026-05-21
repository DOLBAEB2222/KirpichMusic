#!/usr/bin/env bash
# =====================================================================
#  KirpichMusic — setup.sh
#  Полная установка с нуля: PostgreSQL, Go, БД, миграции, сборка, запуск.
#  Поддерживает: apt (Debian/Ubuntu), dnf/yum (Fedora/RHEL/Rocky),
#                pacman (Arch/Manjaro), zypper (openSUSE), apk (Alpine),
#                pkg_add (OpenBSD), brew (macOS).
#  После запуска: ./bin/kirpichmusic — рабочий сервер на :8080.
# =====================================================================
set -euo pipefail

cd "$(dirname "$0")"
ROOT="$(pwd)"

GREEN='\033[1;32m'; YELLOW='\033[1;33m'; RED='\033[1;31m'; CYAN='\033[1;36m'; DIM='\033[2m'; NC='\033[0m'
say()  { printf "${GREEN}▸ %s${NC}\n" "$*"; }
warn() { printf "${YELLOW}⚠ %s${NC}\n" "$*"; }
die()  { printf "${RED}✗ %s${NC}\n" "$*"; exit 1; }
hr()   { printf "${DIM}--------------------------------------------------------------------${NC}\n"; }

# ---- параметры (можно переопределить env'ом) -----------------------
DB_NAME="${DB_NAME:-kirpichmusic}"
DB_USER="${DB_USER:-kirpich}"
DB_PASS="${DB_PASS:-kirpich}"
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"
HTTP_ADDR="${HTTP_ADDR:-:8080}"
GO_MIN_MAJOR=1
GO_MIN_MINOR=22
NO_RUN="${NO_RUN:-0}"   # NO_RUN=1 ./setup.sh — не стартовать сервер по завершении

# ---- detect distro / package manager -------------------------------
PM=""
SUDO=""
need_root_for_install=1
if [[ "$(id -u)" -eq 0 ]]; then need_root_for_install=0; fi
if command -v sudo >/dev/null 2>&1 && [[ $need_root_for_install -eq 1 ]]; then
  SUDO="sudo"
elif command -v doas >/dev/null 2>&1 && [[ $need_root_for_install -eq 1 ]]; then
  SUDO="doas"
fi

OS_ID="$(. /etc/os-release 2>/dev/null && echo "${ID:-}")"
case "$(uname -s)" in
  Darwin)  PM="brew" ;;
  OpenBSD) PM="pkg_add" ;;
  Linux)
    if   command -v apt-get >/dev/null 2>&1; then PM="apt"
    elif command -v dnf      >/dev/null 2>&1; then PM="dnf"
    elif command -v yum      >/dev/null 2>&1; then PM="yum"
    elif command -v pacman   >/dev/null 2>&1; then PM="pacman"
    elif command -v zypper   >/dev/null 2>&1; then PM="zypper"
    elif command -v apk      >/dev/null 2>&1; then PM="apk"
    fi
    ;;
esac
[[ -z "$PM" ]] && warn "Не смог определить пакетный менеджер. Установи psql и go вручную."

say "Дистрибутив: ${OS_ID:-unknown}, пакетный менеджер: ${PM:-(none)}"
hr

# ---- helpers --------------------------------------------------------
pkg_install() {
  case "$PM" in
    apt)    $SUDO apt-get update -y && $SUDO DEBIAN_FRONTEND=noninteractive apt-get install -y "$@" ;;
    dnf)    $SUDO dnf install -y "$@" ;;
    yum)    $SUDO yum install -y "$@" ;;
    pacman) $SUDO pacman -Sy --noconfirm --needed "$@" ;;
    zypper) $SUDO zypper install -y "$@" ;;
    apk)    $SUDO apk add --no-cache "$@" ;;
    pkg_add) $SUDO pkg_add -I "$@" ;;
    brew)   brew install "$@" ;;
    *)      die "Установи вручную: $*" ;;
  esac
}

# ---- 1. Go ----------------------------------------------------------
ensure_go() {
  if command -v go >/dev/null 2>&1; then
    GOVER="$(go version | awk '{print $3}' | sed 's/^go//')"
    GOMAJ="$(echo "$GOVER" | cut -d. -f1)"
    GOMIN="$(echo "$GOVER" | cut -d. -f2)"
    if (( GOMAJ > GO_MIN_MAJOR )) || (( GOMAJ == GO_MIN_MAJOR && GOMIN >= GO_MIN_MINOR )); then
      say "Go ${GOVER} — OK"
      return
    fi
    warn "Go ${GOVER} устарел (нужен ≥${GO_MIN_MAJOR}.${GO_MIN_MINOR}), обновляю…"
  fi
  case "$PM" in
    apt|dnf|yum|pacman|zypper|brew) pkg_install golang-go || pkg_install go || pkg_install golang ;;
    apk) pkg_install go ;;
    pkg_add) pkg_install go ;;
    *) die "Установи Go ≥${GO_MIN_MAJOR}.${GO_MIN_MINOR} вручную: https://go.dev/dl/" ;;
  esac
  command -v go >/dev/null 2>&1 || die "Go не установлен"
}

# ---- 2. PostgreSQL --------------------------------------------------
ensure_pg() {
  if command -v psql >/dev/null 2>&1; then
    say "psql — найден ($(psql --version))"
  else
    say "Ставлю PostgreSQL"
    case "$PM" in
      apt)    pkg_install postgresql postgresql-contrib ;;
      dnf|yum) pkg_install postgresql-server postgresql-contrib postgresql ;;
      pacman) pkg_install postgresql ;;
      zypper) pkg_install postgresql-server postgresql ;;
      apk)    pkg_install postgresql postgresql-contrib ;;
      pkg_add) pkg_install postgresql-server ;;
      brew)   pkg_install postgresql@16 ;;
      *)      die "Установи PostgreSQL вручную" ;;
    esac
  fi

  # Инициализация data-кластера, если нужно (RHEL/Fedora/Arch/OpenBSD).
  case "$PM" in
    dnf|yum)
      if [[ ! -d /var/lib/pgsql/data/base ]]; then
        say "Инициализирую кластер PostgreSQL"
        $SUDO postgresql-setup --initdb 2>/dev/null || $SUDO /usr/bin/postgresql-setup --initdb || true
      fi
      ;;
    pacman)
      if [[ ! -d /var/lib/postgres/data/base ]]; then
        say "Инициализирую кластер PostgreSQL"
        $SUDO -u postgres initdb -D /var/lib/postgres/data --locale=C.UTF-8 --encoding=UTF8 || true
      fi
      ;;
    pkg_add)
      if [[ ! -d /var/postgresql/data/base ]]; then
        say "Инициализирую кластер PostgreSQL"
        $SUDO mkdir -p /var/postgresql/data
        $SUDO chown _postgresql:_postgresql /var/postgresql/data
        $SUDO -u _postgresql initdb -D /var/postgresql/data --locale=C.UTF-8 --encoding=UTF8 || true
      fi
      ;;
  esac

  # Старт сервиса (systemd / brew / apk / rcctl).
  if command -v systemctl >/dev/null 2>&1; then
    $SUDO systemctl enable --now postgresql 2>/dev/null || \
    $SUDO systemctl enable --now postgresql.service 2>/dev/null || true
  elif command -v rc-service >/dev/null 2>&1; then
    $SUDO rc-update add postgresql default 2>/dev/null || true
    $SUDO rc-service postgresql start 2>/dev/null || true
  elif command -v rcctl >/dev/null 2>&1; then
    $SUDO rcctl enable postgresql 2>/dev/null || true
    $SUDO rcctl start postgresql 2>/dev/null || true
  elif [[ "$PM" == "brew" ]]; then
    brew services start postgresql@16 2>/dev/null || brew services start postgresql 2>/dev/null || true
  fi

  # Ждём пока сервер реально слушает.
  for _ in $(seq 1 20); do
    if pg_isready -h "$DB_HOST" -p "$DB_PORT" -q 2>/dev/null; then break; fi
    sleep 0.5
  done
  pg_isready -h "$DB_HOST" -p "$DB_PORT" -q || warn "psql ещё не слушает $DB_HOST:$DB_PORT — продолжаю на свой страх и риск"
}

# ---- 3. БД и роль ---------------------------------------------------
ensure_db() {
  say "Создаю роль и БД ($DB_USER / $DB_NAME)"
  # Используем postgres-суперюзера локально (peer / trust).
  # OpenBSD использует _postgresql, Linux — postgres, macOS — текущего юзера.
  PG_SUPERUSER="postgres"
  if [[ "$(uname -s)" = "OpenBSD" ]]; then
    PG_SUPERUSER="_postgresql"
  fi

  PSQL_SUPER="$SUDO -u $PG_SUPERUSER psql -v ON_ERROR_STOP=1 -h $DB_HOST -p $DB_PORT"
  if ! $SUDO -u $PG_SUPERUSER true 2>/dev/null; then
    # macOS / brew: postgres-юзера нет, работаем под текущим.
    PSQL_SUPER="psql -v ON_ERROR_STOP=1 -h $DB_HOST -p $DB_PORT -d postgres"
  fi
  $PSQL_SUPER <<SQL || warn "psql/createuser операция уже была выполнена (это нормально)"
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}') THEN
    CREATE ROLE "${DB_USER}" LOGIN PASSWORD '${DB_PASS}';
  ELSE
    ALTER ROLE "${DB_USER}" WITH LOGIN PASSWORD '${DB_PASS}';
  END IF;
END\$\$;
SELECT 'create db' WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname='${DB_NAME}')\gexec
\set ON_ERROR_STOP off
CREATE DATABASE "${DB_NAME}" OWNER "${DB_USER}";
\set ON_ERROR_STOP on
GRANT ALL PRIVILEGES ON DATABASE "${DB_NAME}" TO "${DB_USER}";
SQL
}

# ---- 4. Миграции ----------------------------------------------------
run_migrations() {
  say "Применяю миграции"
  shopt -s nullglob
  local files=(db/migrations/*.sql)
  shopt -u nullglob
  if (( ${#files[@]} == 0 )); then
    warn "Нет миграций в db/migrations"
    return
  fi
  for f in "${files[@]}"; do
    printf "  ${CYAN}→${NC} %s\n" "$f"
    PGPASSWORD="$DB_PASS" psql -v ON_ERROR_STOP=1 -h "$DB_HOST" -p "$DB_PORT" \
      -U "$DB_USER" -d "$DB_NAME" -f "$f" >/tmp/km_mig_$(basename "$f").log 2>&1 || {
      warn "Ошибка миграции $f — см. /tmp/km_mig_$(basename "$f").log"
      tail -20 "/tmp/km_mig_$(basename "$f").log" || true
      die "stop on migration $f"
    }
  done
}

# ---- 5. .env --------------------------------------------------------
ensure_env() {
  if [[ ! -f .env ]]; then
    say "Создаю .env"
    cat > .env <<EOF
ENV=dev
HTTP_ADDR=${HTTP_ADDR}
DATABASE_URL=postgres://${DB_USER}:${DB_PASS}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable
SESSION_TTL=720h
SESSION_COOKIE=km_sid
MEDIA_ROOT=./media
MAX_UPLOAD_MB=100
WEB_ROOT=./web
EOF
  else
    say ".env уже есть — оставляю как есть"
  fi
}

# ---- 6. сборка ------------------------------------------------------
build_bin() {
  say "Сборка: go build → bin/kirpichmusic"
  mkdir -p bin
  go build -trimpath -o bin/kirpichmusic ./cmd/server
  ls -la bin/kirpichmusic
}

# ---- 7. финальный запуск -------------------------------------------
hint_admin() {
  cat <<EOF

${GREEN}╔══════════════════════════════════════════════════════════════════╗${NC}
${GREEN}║                    Установка завершена ✓                         ║${NC}
${GREEN}╚══════════════════════════════════════════════════════════════════╝${NC}

${CYAN}Запуск:${NC}        ./run.sh         (или ./bin/kirpichmusic)
${CYAN}Открыть:${NC}       http://localhost${HTTP_ADDR}
${CYAN}Админка:${NC}       http://localhost${HTTP_ADDR}/admin

Первый зарегистрированный пользователь автоматически становится админом.
Чтобы выдать админку существующему юзеру вручную:

  PGPASSWORD=${DB_PASS} psql -h ${DB_HOST} -U ${DB_USER} -d ${DB_NAME} \\
    -c "UPDATE users SET is_admin=TRUE WHERE username='ВАШ_НИК'"

EOF
}

# ─── pipeline ─────────────────────────────────────────────────────
ensure_go
ensure_pg
ensure_db
ensure_env
run_migrations
build_bin
hint_admin

if [[ "$NO_RUN" != "1" ]]; then
  say "Запускаю сервер (Ctrl+C — выход)"
  exec ./bin/kirpichmusic
fi
