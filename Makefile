# Project variables
BINARY_NAME=conductor
MAIN_PACKAGE=./cmd/conductor
DIST_DIR=dist
VERSION_PKG=internal/version/version.go

# Get version from git tags, fallback to dev if no tags exist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u +%Y-%m-%d)

# Go build flags
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
GOFLAGS := -ldflags "$(LDFLAGS)"

# Supported platforms for distribution
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Colors for terminal output
COLOR_RESET = \033[0m
COLOR_CYAN = \033[36m
COLOR_GREEN = \033[32m

.PHONY: all
all: clean build

.PHONY: run
run: ## Run the project locally
	@printf "$(COLOR_CYAN)Running \"$(BINARY_NAME)\"\n"
	@go run "$(MAIN_PACKAGE)/main.go"

.PHONY: set-version
set-version:
	@printf "$(COLOR_CYAN)Setting version information...$(COLOR_RESET)\n"
	@echo "package version\n\nvar (\n    Version = \"$(VERSION)\"\n    Commit = \"$(COMMIT)\"\n    Date = \"$(DATE)\"\n)" > $(VERSION_PKG)
	@printf "$(COLOR_GREEN)Done!$(COLOR_RESET)\n"

.PHONY: build
build: set-version ## Build binary for current platform
	@printf "$(COLOR_CYAN)Building $(BINARY_NAME)...$(COLOR_RESET)\n"
	@go build $(GOFLAGS) -o $(BINARY_NAME) $(MAIN_PACKAGE)
	@printf "$(COLOR_GREEN)Done!$(COLOR_RESET)\n"

.PHONY: install
install: set-version ## Install binary to $GOPATH/bin
	@printf "$(COLOR_CYAN)Installing $(BINARY_NAME)...$(COLOR_RESET)\n"
	@go install $(GOFLAGS) $(MAIN_PACKAGE)
	@printf "$(COLOR_GREEN)Done! Binary installed to $$GOPATH/bin/$(BINARY_NAME)$(COLOR_RESET)\n"

.PHONY: clean
clean: ## Clean build artifacts
	@printf "$(COLOR_CYAN)Cleaning...$(COLOR_RESET)\n"
	@rm -rf $(DIST_DIR)
	@rm -f $(BINARY_NAME)
	@rm -f $(VERSION_PKG)
	@go clean
	@printf "$(COLOR_GREEN)Done!$(COLOR_RESET)\n"

.PHONY: test
test: ## Run tests
	@printf "$(COLOR_CYAN)Running tests...$(COLOR_RESET)\n"
	@go test -v ./...
	@printf "$(COLOR_GREEN)Done!$(COLOR_RESET)\n"

.PHONY: lint
lint: ## Run linters
	@printf "$(COLOR_CYAN)Running linters...$(COLOR_RESET)\n"
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run ./...; \
	else \
		printf "$(COLOR_CYAN)golangci-lint not installed, installing...$(COLOR_RESET)\n"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run ./...; \
	fi
	@printf "$(COLOR_GREEN)Done!$(COLOR_RESET)\n"

.PHONY: fmt
fmt: ## Format code
	@printf "$(COLOR_CYAN)Formatting code...$(COLOR_RESET)\n"
	@go fmt ./...
	@printf "$(COLOR_GREEN)Done!$(COLOR_RESET)\n"

.PHONY: dist
dist: clean set-version ## Build binaries for all supported platforms
	@printf "$(COLOR_CYAN)Building binaries for all platforms...$(COLOR_RESET)\n"
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} \
		GOARCH=$${platform#*/} \
		OUTPUT_NAME=$(DIST_DIR)/$(BINARY_NAME)-$${platform%/*}-$${platform#*/} \
		; \
		if [ "$${platform%/*}" = "windows" ]; then \
			OUTPUT_NAME+='.exe' \
		; \
		fi \
		; \
		printf "$(COLOR_CYAN)Building $${platform%/*}/$${platform#*/}...$(COLOR_RESET)\n" \
		; \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} go build $(GOFLAGS) -o $${OUTPUT_NAME} $(MAIN_PACKAGE) \
		|| exit 1 \
		; \
	done
	@printf "$(COLOR_GREEN)Done! Binaries available in $(DIST_DIR)$(COLOR_RESET)\n"

.PHONY: release
release: lint test dist ## Prepare for release (lint, test, and build distributions)
	@printf "$(COLOR_GREEN)Release preparation complete!$(COLOR_RESET)\n"
	@printf "$(COLOR_CYAN)Version: $(VERSION)$(COLOR_RESET)\n"
	@printf "$(COLOR_CYAN)Commit: $(COMMIT)$(COLOR_RESET)\n"
	@printf "$(COLOR_CYAN)Date: $(DATE)$(COLOR_RESET)\n"
	@printf "$(COLOR_CYAN)Binaries available in $(DIST_DIR)$(COLOR_RESET)\n"

.PHONY: dev
dev: set-version ## Build and run for development
	@printf "$(COLOR_CYAN)Building and running for development...$(COLOR_RESET)\n"
	@go run $(GOFLAGS) $(MAIN_PACKAGE)

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*##"; printf "\033[36m"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-15s %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } END {printf "\033[0m"}' $(MAKEFILE_LIST)

# Set default goal to help
.DEFAULT_GOAL := help
