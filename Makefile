DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable

.PHONY: test test-unit test-integration test-e2e build dev
.PHONY: mcp mcp-test mcp-test-unit mcp-test-integration mcp-test-e2e

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

# txtscape-mcp (committable project memory)
mcp:
	cd txtscape-mcp && go build -o ../bin/txtscape-mcp .

mcp-test: mcp-test-unit mcp-test-integration mcp-test-e2e

mcp-test-unit:
	cd txtscape-mcp && go test -count=1 ./...

mcp-test-integration: mcp
	cd txtscape-mcp && go test -tags=integration -count=1 ./tests/integration/

mcp-test-e2e: mcp
	cd txtscape-mcp && go test -tags=e2e -count=1 ./e2e/
