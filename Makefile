.PHONY: build test lint clean install release krew-manifest help version

# Variables
BINARY_NAME=kubectl-rltop
VERSION=$(shell cat VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(GIT_COMMIT) -X main.date=$(BUILD_DATE)"
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

# Build info
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"

build: ## Build the binary for current platform
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@go build $(LDFLAGS) -o $(BINARY_NAME) .

build-all: ## Build binaries for all platforms
	@echo "Building for all platforms..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 .
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 .
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 .
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 .
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Build complete. Binaries are in dist/"

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint: ## Run linters
	@echo "Running linters..."
	@golangci-lint run

lint-fix: ## Run linters and fix issues
	@echo "Running linters and fixing issues..."
	@golangci-lint run --fix

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf dist/ build/ release/ *.tar.gz *.zip coverage.out coverage.html
	@rm -f $(BINARY_NAME) $(BINARY_NAME).exe
	@echo "Clean complete"

install: build ## Install the binary locally
	@echo "Installing $(BINARY_NAME)..."
	@go install $(LDFLAGS) .
	@echo "Installation complete"

release: clean build-all ## Create release artifacts
	@echo "Creating release artifacts for $(VERSION)..."
	@mkdir -p release
	@cd dist && tar -czf ../release/$(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	@cd dist && tar -czf ../release/$(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	@cd dist && tar -czf ../release/$(BINARY_NAME)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	@cd dist && tar -czf ../release/$(BINARY_NAME)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	@cd dist && zip -q ../release/$(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	@echo "Release artifacts created in release/"

krew-manifest: ## Validate Krew plugin manifest
	@echo "Validating plugin.yaml..."
	@which krew > /dev/null || (echo "krew not found. Install from https://krew.sigs.k8s.io/" && exit 1)
	@krew plugin lint plugin.yaml || true
	@echo "Manifest validation complete"

verify: test lint ## Run all verification checks
	@echo "All checks passed!"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

update-deps: ## Update dependencies
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

