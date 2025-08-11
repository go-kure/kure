# Kure Makefile - A Go library for programmatically building Kubernetes resources for GitOps tools
# Provides standardized commands for building, testing, linting, and development workflows

# Go configuration
GO := go
GOROOT ?= $(shell go env GOROOT)
GOPATH ?= $(shell go env GOPATH)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Project configuration
MODULE := $(shell head -1 go.mod | awk '{print $$2}')
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build configuration
BUILD_DIR := bin
OUTPUT_DIR := out
COVERAGE_DIR := coverage

# Executables
KURE_BIN := $(BUILD_DIR)/kure
KUREL_BIN := $(BUILD_DIR)/kurel
DEMO_BIN := $(BUILD_DIR)/demo

# Test configuration
TEST_TIMEOUT := 30s
TEST_PACKAGES := ./...
COVERAGE_THRESHOLD := 80

# Linting configuration
GOLANGCI_LINT_VERSION := v1.64.8

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m
COLOR_RED := \033[31m

.PHONY: help
help: ## Display this help message
	@echo "$(COLOR_BOLD)Kure Makefile Commands$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)A Go library for programmatically building Kubernetes resources for GitOps tools$(COLOR_RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "$(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: all
all: clean deps build test lint ## Run all standard development tasks

.PHONY: info
info: ## Display project information
	@echo "$(COLOR_BOLD)Project Information$(COLOR_RESET)"
	@echo "Module:      $(MODULE)"
	@echo "Version:     $(VERSION)"
	@echo "Git Commit:  $(GIT_COMMIT)"
	@echo "Build Time:  $(BUILD_TIME)"
	@echo "Go Version:  $(shell $(GO) version)"
	@echo "GOOS:        $(GOOS)"
	@echo "GOARCH:      $(GOARCH)"

# =============================================================================
# Dependencies
# =============================================================================

.PHONY: deps
deps: ## Download and tidy Go modules
	@echo "$(COLOR_YELLOW)Downloading dependencies...$(COLOR_RESET)"
	$(GO) mod download
	$(GO) mod tidy
	@echo "$(COLOR_GREEN)Dependencies updated$(COLOR_RESET)"

.PHONY: deps-upgrade
deps-upgrade: ## Upgrade all dependencies to latest versions
	@echo "$(COLOR_YELLOW)Upgrading dependencies...$(COLOR_RESET)"
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "$(COLOR_GREEN)Dependencies upgraded$(COLOR_RESET)"

# =============================================================================
# Building
# =============================================================================

.PHONY: build
build: build-kure build-kurel build-demo ## Build all executables

.PHONY: build-kure
build-kure: $(BUILD_DIR) ## Build the kure executable
	@echo "$(COLOR_YELLOW)Building kure...$(COLOR_RESET)"
	$(GO) build -ldflags="-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(KURE_BIN) ./cmd/kure
	@echo "$(COLOR_GREEN)Built $(KURE_BIN)$(COLOR_RESET)"

.PHONY: build-kurel
build-kurel: $(BUILD_DIR) ## Build the kurel executable
	@echo "$(COLOR_YELLOW)Building kurel...$(COLOR_RESET)"
	$(GO) build -ldflags="-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(KUREL_BIN) ./cmd/kurel
	@echo "$(COLOR_GREEN)Built $(KUREL_BIN)$(COLOR_RESET)"

.PHONY: build-demo
build-demo: $(BUILD_DIR) ## Build the demo executable
	@echo "$(COLOR_YELLOW)Building demo...$(COLOR_RESET)"
	$(GO) build -ldflags="-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(DEMO_BIN) ./cmd/demo
	@echo "$(COLOR_GREEN)Built $(DEMO_BIN)$(COLOR_RESET)"

.PHONY: build-race
build-race: $(BUILD_DIR) ## Build all executables with race detection
	@echo "$(COLOR_YELLOW)Building with race detection...$(COLOR_RESET)"
	$(GO) build -race -o $(KURE_BIN) ./cmd/kure
	$(GO) build -race -o $(KUREL_BIN) ./cmd/kurel
	$(GO) build -race -o $(DEMO_BIN) ./cmd/demo
	@echo "$(COLOR_GREEN)Built all executables with race detection$(COLOR_RESET)"

$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

# =============================================================================
# Testing
# =============================================================================

.PHONY: test
test: ## Run all tests
	@echo "$(COLOR_YELLOW)Running tests...$(COLOR_RESET)"
	$(GO) test -timeout $(TEST_TIMEOUT) $(TEST_PACKAGES)
	@echo "$(COLOR_GREEN)All tests passed$(COLOR_RESET)"

.PHONY: test-verbose
test-verbose: ## Run all tests with verbose output
	@echo "$(COLOR_YELLOW)Running tests with verbose output...$(COLOR_RESET)"
	$(GO) test -v -timeout $(TEST_TIMEOUT) $(TEST_PACKAGES)

.PHONY: test-race
test-race: ## Run tests with race detection
	@echo "$(COLOR_YELLOW)Running tests with race detection...$(COLOR_RESET)"
	$(GO) test -race -timeout $(TEST_TIMEOUT) $(TEST_PACKAGES)
	@echo "$(COLOR_GREEN)All race tests passed$(COLOR_RESET)"

.PHONY: test-short
test-short: ## Run short tests only
	@echo "$(COLOR_YELLOW)Running short tests...$(COLOR_RESET)"
	$(GO) test -short -timeout $(TEST_TIMEOUT) $(TEST_PACKAGES)
	@echo "$(COLOR_GREEN)Short tests passed$(COLOR_RESET)"

.PHONY: test-coverage
test-coverage: $(COVERAGE_DIR) ## Run tests with coverage report
	@echo "$(COLOR_YELLOW)Running tests with coverage...$(COLOR_RESET)"
	$(GO) test -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic $(TEST_PACKAGES)
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	$(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -1
	@echo "$(COLOR_GREEN)Coverage report generated: $(COVERAGE_DIR)/coverage.html$(COLOR_RESET)"

.PHONY: test-benchmark
test-benchmark: ## Run benchmark tests
	@echo "$(COLOR_YELLOW)Running benchmark tests...$(COLOR_RESET)"
	$(GO) test -bench=. -benchmem $(TEST_PACKAGES)

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(COLOR_YELLOW)Running integration tests...$(COLOR_RESET)"
	$(GO) test -tags=integration -timeout 5m $(TEST_PACKAGES)

$(COVERAGE_DIR):
	@mkdir -p $(COVERAGE_DIR)

# =============================================================================
# Code Quality
# =============================================================================

.PHONY: lint
lint: lint-go ## Run all linters

.PHONY: lint-go
lint-go: ## Run Go linting with golangci-lint
	@echo "$(COLOR_YELLOW)Running Go linting...$(COLOR_RESET)"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "$(COLOR_RED)golangci-lint not found. Installing...$(COLOR_RESET)"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
	fi
	golangci-lint run --timeout=10m ./...
	@echo "$(COLOR_GREEN)Go linting passed$(COLOR_RESET)"

.PHONY: fmt
fmt: ## Format Go code
	@echo "$(COLOR_YELLOW)Formatting Go code...$(COLOR_RESET)"
	$(GO) fmt ./...
	@echo "$(COLOR_GREEN)Code formatted$(COLOR_RESET)"

.PHONY: vet
vet: ## Run go vet (excluding copylocks for existing code)
	@echo "$(COLOR_YELLOW)Running go vet...$(COLOR_RESET)"
	$(GO) vet -copylocks=false ./...
	@echo "$(COLOR_GREEN)Go vet completed$(COLOR_RESET)"

.PHONY: tidy
tidy: ## Tidy up go modules
	@echo "$(COLOR_YELLOW)Tidying modules...$(COLOR_RESET)"
	$(GO) mod tidy
	@echo "$(COLOR_GREEN)Modules tidied$(COLOR_RESET)"

.PHONY: qodana
qodana: ## Run Qodana static analysis (requires Docker)
	@echo "$(COLOR_YELLOW)Running Qodana analysis...$(COLOR_RESET)"
	@if ! command -v docker >/dev/null 2>&1; then \
		echo "$(COLOR_RED)Docker not found. Qodana requires Docker to run.$(COLOR_RESET)"; \
		exit 1; \
	fi
	docker run --rm -it -p 8080:8080 \
		-v $(PWD):/data/project:cached \
		jetbrains/qodana-go:2025.1 --show-report
	@echo "$(COLOR_GREEN)Qodana analysis completed$(COLOR_RESET)"

# =============================================================================
# Demo and Examples
# =============================================================================

.PHONY: demo
demo: build-demo $(OUTPUT_DIR) ## Run the comprehensive demo
	@echo "$(COLOR_YELLOW)Running Kure demo...$(COLOR_RESET)"
	$(DEMO_BIN)
	@echo "$(COLOR_GREEN)Demo completed$(COLOR_RESET)"

.PHONY: demo-internals
demo-internals: build-demo $(OUTPUT_DIR) ## Run demo with internal API examples
	@echo "$(COLOR_YELLOW)Running internal API demo...$(COLOR_RESET)"
	$(DEMO_BIN) --internals
	@echo "$(COLOR_GREEN)Internal API demo completed$(COLOR_RESET)"

.PHONY: demo-gvk
demo-gvk: build-demo $(OUTPUT_DIR) ## Run GVK generators demo
	@echo "$(COLOR_YELLOW)Running GVK generators demo...$(COLOR_RESET)"
	$(DEMO_BIN) --gvk
	@echo "$(COLOR_GREEN)GVK demo completed$(COLOR_RESET)"

.PHONY: examples
examples: demo ## Generate all example outputs (alias for demo)

$(OUTPUT_DIR):
	@mkdir -p $(OUTPUT_DIR)

# =============================================================================
# Package Operations
# =============================================================================

.PHONY: kurel-build
kurel-build: build-kurel ## Build a kurel package (requires PACKAGE_PATH)
	@if [ -z "$(PACKAGE_PATH)" ]; then \
		echo "$(COLOR_RED)Error: PACKAGE_PATH is required$(COLOR_RESET)"; \
		echo "Usage: make kurel-build PACKAGE_PATH=path/to/package"; \
		exit 1; \
	fi
	@echo "$(COLOR_YELLOW)Building kurel package: $(PACKAGE_PATH)$(COLOR_RESET)"
	$(KUREL_BIN) build $(PACKAGE_PATH)
	@echo "$(COLOR_GREEN)Package built successfully$(COLOR_RESET)"

.PHONY: kurel-info
kurel-info: build-kurel ## Show package information (requires PACKAGE_PATH)
	@if [ -z "$(PACKAGE_PATH)" ]; then \
		echo "$(COLOR_RED)Error: PACKAGE_PATH is required$(COLOR_RESET)"; \
		echo "Usage: make kurel-info PACKAGE_PATH=path/to/package"; \
		exit 1; \
	fi
	@echo "$(COLOR_YELLOW)Package information: $(PACKAGE_PATH)$(COLOR_RESET)"
	$(KUREL_BIN) info $(PACKAGE_PATH)

# =============================================================================
# Development Utilities
# =============================================================================

.PHONY: tools
tools: ## Install development tools
	@echo "$(COLOR_YELLOW)Installing development tools...$(COLOR_RESET)"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
		echo "Installed golangci-lint $(GOLANGCI_LINT_VERSION)"; \
	fi
	@echo "$(COLOR_GREEN)Development tools installed$(COLOR_RESET)"

.PHONY: generate
generate: ## Run go generate for all packages
	@echo "$(COLOR_YELLOW)Running go generate...$(COLOR_RESET)"
	$(GO) generate ./...
	@echo "$(COLOR_GREEN)Code generation completed$(COLOR_RESET)"

.PHONY: mod-graph
mod-graph: ## Display module dependency graph
	@echo "$(COLOR_YELLOW)Module dependency graph:$(COLOR_RESET)"
	$(GO) mod graph | head -20

.PHONY: list-packages
list-packages: ## List all packages in the module
	@echo "$(COLOR_YELLOW)Module packages:$(COLOR_RESET)"
	$(GO) list ./...

.PHONY: outdated
outdated: ## Check for outdated dependencies
	@echo "$(COLOR_YELLOW)Checking for outdated dependencies...$(COLOR_RESET)"
	$(GO) list -u -m all | grep '\[' || echo "$(COLOR_GREEN)All dependencies are up to date$(COLOR_RESET)"

# =============================================================================
# CI/CD
# =============================================================================

.PHONY: ci
ci: deps lint test build ## Run CI pipeline tasks

.PHONY: ci-coverage
ci-coverage: deps lint test-coverage build ## Run CI pipeline with coverage

.PHONY: ci-integration
ci-integration: deps lint test test-integration build ## Run CI pipeline with integration tests

.PHONY: check
check: lint vet test-short ## Quick code quality check

.PHONY: precommit
precommit: fmt tidy lint vet test ## Run all pre-commit checks

# =============================================================================
# Cleanup
# =============================================================================

.PHONY: clean
clean: ## Clean build artifacts and caches
	@echo "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	rm -rf $(BUILD_DIR) $(OUTPUT_DIR) $(COVERAGE_DIR)
	$(GO) clean -cache -testcache -modcache
	@echo "$(COLOR_GREEN)Cleanup completed$(COLOR_RESET)"

.PHONY: clean-build
clean-build: ## Clean only build artifacts
	@echo "$(COLOR_YELLOW)Cleaning build directory...$(COLOR_RESET)"
	rm -rf $(BUILD_DIR)
	@echo "$(COLOR_GREEN)Build directory cleaned$(COLOR_RESET)"

.PHONY: clean-output
clean-output: ## Clean only output directory
	@echo "$(COLOR_YELLOW)Cleaning output directory...$(COLOR_RESET)"
	rm -rf $(OUTPUT_DIR)
	@echo "$(COLOR_GREEN)Output directory cleaned$(COLOR_RESET)"

.PHONY: clean-cache
clean-cache: ## Clean Go caches
	@echo "$(COLOR_YELLOW)Cleaning Go caches...$(COLOR_RESET)"
	$(GO) clean -cache -testcache -modcache
	@echo "$(COLOR_GREEN)Caches cleaned$(COLOR_RESET)"

# =============================================================================
# Release
# =============================================================================

.PHONY: release-check
release-check: ## Check if ready for release
	@echo "$(COLOR_YELLOW)Checking release readiness...$(COLOR_RESET)"
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "$(COLOR_RED)Error: Working directory is not clean$(COLOR_RESET)"; \
		git status --porcelain; \
		exit 1; \
	fi
	@if ! git diff --quiet HEAD~1; then \
		echo "$(COLOR_GREEN)Changes detected since last commit$(COLOR_RESET)"; \
	fi
	@echo "$(COLOR_GREEN)Ready for release$(COLOR_RESET)"

.PHONY: release-build
release-build: clean deps ci ## Build release artifacts
	@echo "$(COLOR_YELLOW)Building release artifacts...$(COLOR_RESET)"
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(BUILD_DIR)/kure-linux-amd64 ./cmd/kure
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(BUILD_DIR)/kurel-linux-amd64 ./cmd/kurel
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(BUILD_DIR)/kure-darwin-amd64 ./cmd/kure
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(BUILD_DIR)/kurel-darwin-amd64 ./cmd/kurel
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(BUILD_DIR)/kure-darwin-arm64 ./cmd/kure
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(BUILD_DIR)/kurel-darwin-arm64 ./cmd/kurel
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(BUILD_DIR)/kure-windows-amd64.exe ./cmd/kure
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(BUILD_DIR)/kurel-windows-amd64.exe ./cmd/kurel
	@echo "$(COLOR_GREEN)Release artifacts built in $(BUILD_DIR)/$(COLOR_RESET)"
	@ls -la $(BUILD_DIR)/

# =============================================================================
# Default target
# =============================================================================

.DEFAULT_GOAL := help