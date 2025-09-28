# Makefile for cross-platform builds of file-server

BINARY_NAME=fileserver
VERSION=1.0.0
BUILD_DIR=build

# Build for current platform
build:
	go build -o $(BINARY_NAME) fileserver.go

# Cross-compile for Linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 fileserver.go

# Cross-compile for macOS
build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 fileserver.go

# Cross-compile for Windows
build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe fileserver.go

# Build all platforms
build-all: build-linux build-darwin build-windows

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)

.PHONY: build build-linux build-darwin build-windows build-all clean