MIGRATE := $(HOME)/go/bin/migrate
DB_URL   := postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable

.PHONY: migrate-up migrate-down build test

migrate-up:
	$(MIGRATE) -path internal/db/migrations -database "$(DB_URL)" up

migrate-down:
	$(MIGRATE) -path internal/db/migrations -database "$(DB_URL)" down

build:
	go build ./...

test:
	go test ./... -v
