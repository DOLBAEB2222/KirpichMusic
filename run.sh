#!/usr/bin/env bash
# =====================================================================
#  KirpichMusic — run.sh
#  Простой запуск сервера.
#  Подразумевает что setup.sh уже отработал: бинарь собран, БД настроена.
#  Если бинаря нет — пересоберёт.
# =====================================================================
set -euo pipefail
cd "$(dirname "$0")"

if [[ ! -f bin/kirpichmusic ]]; then
  echo "▸ Бинарь bin/kirpichmusic отсутствует — собираю"
  if ! command -v go >/dev/null 2>&1; then
    echo "✗ Go не установлен. Запусти ./setup.sh — он всё поставит."
    exit 1
  fi
  mkdir -p bin
  go build -trimpath -o bin/kirpichmusic ./cmd/server
fi

if [[ ! -f .env ]]; then
  echo "✗ .env не найден. Сначала запусти ./setup.sh"
  exit 1
fi

# Сам бинарь умеет читать .env из текущей папки (см. internal/config/dotenv.go),
# но на всякий случай экспортируем переменные и в shell — для psql/migrate, если
# они вызываются параллельно.
set -a
# shellcheck disable=SC1091
source .env
set +a

exec ./bin/kirpichmusic
