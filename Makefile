# Makefile for ttools

# Variables
BINARY_NAME=ttools
MAIN_PKG=./cmd/ttools
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w"
DIST_DIR=dist
BIN_EXT=

# Detect OS for binary extension
ifeq ($(OS),Windows_NT)
    BIN_EXT=.exe
endif

# Build for current platform
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) for current platform..."
	@go build -o $(BINARY_NAME)$(BIN_EXT) $(MAIN_PKG)
	@echo "Build complete: $(BINARY_NAME)$(BIN_EXT)"

# Development build (with debug symbols, faster compile)
.PHONY: dev
dev:
	@echo "Building development version..."
	@go build -o $(BINARY_NAME)$(BIN_EXT) $(MAIN_PKG)
	@echo "Dev build complete: $(BINARY_NAME)$(BIN_EXT)"

# Quick run
.PHONY: run
run: dev
	@./$(BINARY_NAME)$(BIN_EXT) $(ARGS)

# Build for Linux (amd64 and arm64)
.PHONY: build/linux
build/linux:
	@echo "Building for Linux..."
	@mkdir -p $(DIST_DIR)/linux-amd64
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/linux-amd64/$(BINARY_NAME) $(MAIN_PKG)
	@echo "  ✓ Linux amd64: $(DIST_DIR)/linux-amd64/$(BINARY_NAME)"
	@mkdir -p $(DIST_DIR)/linux-arm64
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/linux-arm64/$(BINARY_NAME) $(MAIN_PKG)
	@echo "  ✓ Linux arm64: $(DIST_DIR)/linux-arm64/$(BINARY_NAME)"

# Build for Windows (amd64)
.PHONY: build/windows
build/windows:
	@echo "Building for Windows..."
	@mkdir -p $(DIST_DIR)/windows-amd64
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/windows-amd64/$(BINARY_NAME).exe $(MAIN_PKG)
	@echo "  ✓ Windows amd64: $(DIST_DIR)/windows-amd64/$(BINARY_NAME).exe"

# Build for macOS (amd64 and arm64/Apple Silicon)
.PHONY: build/darwin
build/darwin:
	@echo "Building for macOS..."
	@mkdir -p $(DIST_DIR)/darwin-amd64
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/darwin-amd64/$(BINARY_NAME) $(MAIN_PKG)
	@echo "  ✓ macOS amd64: $(DIST_DIR)/darwin-amd64/$(BINARY_NAME)"
	@mkdir -p $(DIST_DIR)/darwin-arm64
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/darwin-arm64/$(BINARY_NAME) $(MAIN_PKG)
	@echo "  ✓ macOS arm64: $(DIST_DIR)/darwin-arm64/$(BINARY_NAME)"

# Build for all platforms
.PHONY: build/all
build/all: build/linux build/windows build/darwin
	@echo ""
	@echo "All builds complete! Binaries available in $(DIST_DIR)/"
	@ls -lh $(DIST_DIR)/*/ 2>/dev/null || true

# Run tests
.PHONY: test
test:
	@go test -v ./...

# Run tests with coverage
.PHONY: coverage
coverage:
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(DIST_DIR)
	@rm -f $(BINARY_NAME)$(BIN_EXT)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# Install to /usr/local/bin
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

# Uninstall from /usr/local/bin
.PHONY: uninstall
uninstall:
	@echo "Uninstalling $(BINARY_NAME) from /usr/local/bin..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstall complete"

# Download and tidy dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated"

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete"

# Display help
.PHONY: help
help:
	@echo "ttools Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build Targets:"
	@echo "  build         Build for current platform"
	@echo "  build/linux   Build for Linux (amd64, arm64)"
	@echo "  build/windows Build for Windows (amd64)"
	@echo "  build/darwin  Build for macOS (amd64, arm64)"
	@echo "  build/all     Build for all platforms"
	@echo ""
	@echo "Development Targets:"
	@echo "  dev           Quick development build"
	@echo "  run           Build and run (use ARGS='...' for arguments)"
	@echo "  test          Run tests"
	@echo "  coverage      Run tests with coverage report"
	@echo "  fmt           Format code with gofmt"
	@echo ""
	@echo "Maintenance Targets:"
	@echo "  clean         Remove build artifacts"
	@echo "  install       Install to /usr/local/bin"
	@echo "  uninstall     Remove from /usr/local/bin"
	@echo "  deps          Download and tidy dependencies"
	@echo "  help          Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build/all           # Build for all platforms"
	@echo "  make run ARGS='--help'   # Run with arguments"
	@echo "  make install             # Install locally"

.DEFAULT_GOAL := help
