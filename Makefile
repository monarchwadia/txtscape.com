DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable

.PHONY: test test-unit test-integration test-e2e build dev deploy

test: test-unit test-integration test-e2e

test-unit:
	go test -count=1 ./...

test-integration:
	DATABASE_URL="$(DATABASE_URL)" go test -tags=integration -count=1 ./tests/integration/

test-e2e:
	DATABASE_URL="$(DATABASE_URL)" go test -tags=e2e -count=1 ./e2e/

build:
	go build -o bin/txtscape ./cmd/txtscape

dev: build
	DATABASE_URL="$(DATABASE_URL)" bin/txtscape

deploy:
	go run scripts/deploy.go
