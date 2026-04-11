
.PHONY: test build dev
.PHONY: mcp mcp-test mcp-test-unit mcp-test-integration mcp-test-e2e
.PHONY: mcp-version mcp-version-check mcp-version-set

build:
	go build -o bin/txtscape ./cmd/txtscape

dev: build
	bin/txtscape

# txtscape-mcp (committable project memory)
mcp:
	cd txtscape-mcp && go build -o ../bin/txtscape-mcp .

# build all platform binaries into the npm package
mcp-npm:
	cd txtscape-mcp && \
	  GOOS=darwin  GOARCH=arm64 go build -trimpath -o npm/txtscape-mcp/bin/txtscape-mcp-darwin-arm64 . && \
	  GOOS=darwin  GOARCH=amd64 go build -trimpath -o npm/txtscape-mcp/bin/txtscape-mcp-darwin-x64 . && \
	  GOOS=linux   GOARCH=amd64 go build -trimpath -o npm/txtscape-mcp/bin/txtscape-mcp-linux-x64 . && \
	  GOOS=linux   GOARCH=arm64 go build -trimpath -o npm/txtscape-mcp/bin/txtscape-mcp-linux-arm64 . && \
	  GOOS=windows GOARCH=amd64 go build -trimpath -o npm/txtscape-mcp/bin/txtscape-mcp-win32-x64.exe .

mcp-test: mcp-test-unit mcp-test-integration mcp-test-e2e

mcp-test-unit:
	cd txtscape-mcp && go test -count=1 ./...

mcp-test-integration: mcp
	cd txtscape-mcp && go test -tags=integration -count=1 ./tests/integration/

mcp-test-e2e: mcp
	cd txtscape-mcp && go test -tags=e2e -count=1 ./e2e/

# version management — reads/writes version across main.go and package.json
mcp-version:
	@cd txtscape-mcp && go run ./cmd/version get

mcp-version-check:
	@cd txtscape-mcp && go run ./cmd/version check

mcp-version-set:
	cd txtscape-mcp && go run ./cmd/version set
