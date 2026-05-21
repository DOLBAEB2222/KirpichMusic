.PHONY: build run tidy migrate fmt vet test

# Если есть .env — экспортируем его в окружение make.
# Это нужно для целей migrate/test, которым нужен DATABASE_URL.
# Сам бинарь (run) умеет читать .env сам, см. config.LoadDotEnv.
ifneq (,$(wildcard ./.env))
	include .env
	export
endif

build:
	go build -trimpath -o bin/kirpichmusic ./cmd/server

run: build
	./bin/kirpichmusic

tidy:
	go mod tidy

fmt:
	gofmt -s -w .

vet:
	go vet ./...

migrate:
	psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/migrations/001_init.sql
	psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/migrations/002_follows_and_waveform.sql

test:
	go test ./...
