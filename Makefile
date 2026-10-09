.PHONY: all build test clean cross-compile lint

BINARY_NAME=nvsh
VERSION=1.0.0
BUILD_DIR=dist

all: test build

build:
	go build -ldflags="-s -w -X 'github.com/fongo/node-version-shifter/pkg/config.AppVersion=$(VERSION)'" -o $(BINARY_NAME) ./cmd/nvsh

test:
	go test -v ./...

cross-compile:
	@mkdir -p $(BUILD_DIR)
	# Linux (amd64 & arm64)
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/nvsh
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/nvsh
	# macOS (Apple Silicon arm64 & Intel amd64)
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/nvsh
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/nvsh
	# Windows (x64 & arm64)
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/nvsh
	GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe ./cmd/nvsh
	@echo "Cross-compilation completed in $(BUILD_DIR)/"

clean:
	rm -rf $(BINARY_NAME) $(BUILD_DIR)
