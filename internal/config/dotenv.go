package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv читает переменные из .env-файла в текущей рабочей директории
// (если он есть) и выставляет их через os.Setenv — НО только те, что ещё
// не заданы в окружении. Так настоящие env-переменные имеют приоритет
// над .env, и .env работает как fallback для локальной разработки.
//
// Поддерживается минимальный синтаксис:
//   KEY=value
//   KEY="value with spaces"
//   KEY='value'
//   # comment line
//
// Без зависимостей. Ошибки чтения игнорируются (файла может не быть —
// это нормально, например в проде переменные приходят из systemd/k8s).
func LoadDotEnv(path string) {
	if path == "" {
		path = ".env"
	}
	f, err := os.Open(path)
	if err != nil {
		return // .env отсутствует — это допустимый случай
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// inline-комментарий после значения вида: KEY=value # note
		// поддерживаем только если value НЕ в кавычках (см. ниже).

		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue // некорректная строка — пропускаем
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])

		// Снимаем кавычки, если есть.
		if len(val) >= 2 {
			first, last := val[0], val[len(val)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				val = val[1 : len(val)-1]
			} else {
				// без кавычек — обрезаем хвостовой комментарий "# ..."
				if i := strings.Index(val, " #"); i >= 0 {
					val = strings.TrimSpace(val[:i])
				}
			}
		}

		// Уважаем уже выставленные env-переменные (в т.ч. пустые? нет —
		// если переменная "выставлена" в пустую строку, считаем что
		// её нет и подменяем из .env).
		if existing, ok := os.LookupEnv(key); ok && existing != "" {
			continue
		}
		_ = os.Setenv(key, val)
	}
}
