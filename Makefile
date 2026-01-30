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

.PHONY: outdated
outdated: ## List outdated Go dependencies
	@echo "$(COLOR_YELLOW)Checking for outdated dependencies...$(COLOR_RESET)"
	$(GO) list -m -u -json all 2>/dev/null | \
		python3 -c "import sys,json; [print(f\"{m['Path']}: {m['Version']} -> {m['Update']['Version']}\") for line in sys.stdin.read().split('}\n{') for m in [json.loads('{' + line.strip().strip('{}').strip() + '}')] if 'Update' in m]" 2>/dev/null || \
		$(GO) list -m -u all 2>/dev/null | grep '\[' || echo "All dependencies are up to date"
	@echo "$(COLOR_GREEN)Dependency check completed$(COLOR_RESET)"

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

.PHONY: test-short
test-short: ## Run short tests only (quick feedback)
	@echo "$(COLOR_YELLOW)Running short tests...$(COLOR_RESET)"
	$(GO) test -short -timeout $(TEST_TIMEOUT) $(TEST_PACKAGES)
	@echo "$(COLOR_GREEN)Short tests passed$(COLOR_RESET)"

.PHONY: test-race
test-race: ## Run tests with race detection
	@echo "$(COLOR_YELLOW)Running tests with race detection...$(COLOR_RESET)"
	$(GO) test -race -timeout $(TEST_TIMEOUT) $(TEST_PACKAGES)
	@echo "$(COLOR_GREEN)All race tests passed$(COLOR_RESET)"

.PHONY: test-coverage
test-coverage: $(COVERAGE_DIR) ## Run tests with coverage report
	@echo "$(COLOR_YELLOW)Running tests with coverage...$(COLOR_RESET)"
	$(GO) test -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic $(TEST_PACKAGES)
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	$(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -1
	@echo "$(COLOR_GREEN)Coverage report generated: $(COVERAGE_DIR)/coverage.html$(COLOR_RESET)"

.PHONY: test-benchmark
test-benchmark: ## Run benchmark tests
	@echo "$(COLOR_YELLOW)Running benchmarks...$(COLOR_RESET)"
	$(GO) test -bench=. -benchmem -run=^$$ -timeout 5m $(TEST_PACKAGES)
	@echo "$(COLOR_GREEN)Benchmarks completed$(COLOR_RESET)"

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(COLOR_YELLOW)Running integration tests...$(COLOR_RESET)"
	$(GO) test -tags=integration -timeout 5m $(TEST_PACKAGES)

.PHONY: test-fuzz
test-fuzz: ## Run fuzz tests for patch parser
	@echo "$(COLOR_YELLOW)Running fuzz tests...$(COLOR_RESET)"
	$(GO) test -fuzz=Fuzz -fuzztime=30s ./pkg/patch/...
	@echo "$(COLOR_GREEN)Fuzz tests completed$(COLOR_RESET)"

$(COVERAGE_DIR):
	@mkdir -p $(COVERAGE_DIR)

# =============================================================================
# Code Quality
# =============================================================================

.PHONY: lint
lint: ## Run linters with golangci-lint
	@echo "$(COLOR_YELLOW)Running linting...$(COLOR_RESET)"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "$(COLOR_RED)golangci-lint not found. Installing...$(COLOR_RESET)"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
	fi
	@PATH="$$(go env GOPATH)/bin:$$PATH" golangci-lint run --timeout=10m ./...
	@echo "$(COLOR_GREEN)Linting passed$(COLOR_RESET)"

.PHONY: fmt
fmt: ## Format Go code
	@echo "$(COLOR_YELLOW)Formatting Go code...$(COLOR_RESET)"
	$(GO) fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		PATH="$$(go env GOPATH)/bin:$$PATH" goimports -w . || echo "$(COLOR_RED)goimports not found, run 'make tools' to install$(COLOR_RESET)"; \
	fi
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

.PHONY: vuln
vuln: ## Run vulnerability check with govulncheck
	@echo "$(COLOR_YELLOW)Running vulnerability check...$(COLOR_RESET)"
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "$(COLOR_YELLOW)Installing govulncheck...$(COLOR_RESET)"; \
		$(GO) install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	@PATH="$$(go env GOPATH)/bin:$$PATH" govulncheck ./...
	@echo "$(COLOR_GREEN)Vulnerability check completed$(COLOR_RESET)"

# =============================================================================
# Demo and Examples
# =============================================================================

.PHONY: demo
demo: build-demo $(OUTPUT_DIR) ## Run the comprehensive demo
	@echo "$(COLOR_YELLOW)Running Kure demo...$(COLOR_RESET)"
	$(DEMO_BIN)
	@echo "$(COLOR_GREEN)Demo completed$(COLOR_RESET)"

$(OUTPUT_DIR):
	@mkdir -p $(OUTPUT_DIR)

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
	@if ! command -v goimports >/dev/null 2>&1; then \
		$(GO) install golang.org/x/tools/cmd/goimports@latest; \
		echo "Installed goimports"; \
	fi
	@if ! command -v goreleaser >/dev/null 2>&1; then \
		$(GO) install github.com/goreleaser/goreleaser/v2@latest; \
		echo "Installed goreleaser v2"; \
	fi
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		$(GO) install golang.org/x/vuln/cmd/govulncheck@latest; \
		echo "Installed govulncheck"; \
	fi
	@echo "$(COLOR_GREEN)Development tools installed$(COLOR_RESET)"

.PHONY: sync-go-version
sync-go-version: ## Sync Go version from mise.toml to all files
	@echo "$(COLOR_YELLOW)Syncing Go version from mise.toml...$(COLOR_RESET)"
	@if [ ! -f mise.toml ]; then \
		echo "$(COLOR_RED)Error: mise.toml not found$(COLOR_RESET)"; \
		exit 1; \
	fi
	@GO_VER=$$(grep '^go = ' mise.toml | cut -d'"' -f2); \
	if [ -z "$$GO_VER" ]; then \
		echo "$(COLOR_RED)Error: Could not extract Go version from mise.toml$(COLOR_RESET)"; \
		exit 1; \
	fi; \
	echo "Syncing to Go version: $$GO_VER"; \
	sed -i "s/GO_VERSION: '[^']*'/GO_VERSION: '$$GO_VER'/" .github/workflows/*.yml .github/workflows/*.yaml; \
	sed -i "s/go-version: '[^']*'/go-version: '$$GO_VER'/" .github/workflows/*.yml .github/workflows/*.yaml; \
	sed -i "s/go-version: \$${{ env.GO_VERSION }}/go-version: \$${{ env.GO_VERSION }}/" .github/workflows/*.yml .github/workflows/*.yaml; \
	sed -i "3s/go .*/go $$GO_VER/" go.mod; \
	if [ -f docs/github-workflows.md ]; then \
		sed -i "s/Go Version: \`[0-9][^']*\`/Go Version: \`$$GO_VER\`/g" docs/github-workflows.md; \
	fi; \
	echo "$(COLOR_GREEN)Go version synced to $$GO_VER$(COLOR_RESET)"

.PHONY: check-go-version
check-go-version: ## Verify Go version consistency across all files
	@echo "$(COLOR_YELLOW)Checking Go version consistency...$(COLOR_RESET)"
	@if [ ! -f mise.toml ]; then \
		echo "$(COLOR_RED)Error: mise.toml not found$(COLOR_RESET)"; \
		exit 1; \
	fi
	@GO_VER=$$(grep '^go = ' mise.toml | cut -d'"' -f2); \
	if [ -z "$$GO_VER" ]; then \
		echo "$(COLOR_RED)Error: Could not extract Go version from mise.toml$(COLOR_RESET)"; \
		exit 1; \
	fi; \
	echo "Expected Go version: $$GO_VER"; \
	ERRORS=0; \
	for file in .github/workflows/*.yml .github/workflows/*.yaml; do \
		if [ -f "$$file" ]; then \
			if grep -q "GO_VERSION:" "$$file"; then \
				FILE_VER=$$(grep "GO_VERSION:" "$$file" | head -1 | cut -d"'" -f2); \
				if [ "$$FILE_VER" != "$$GO_VER" ]; then \
					echo "$(COLOR_RED)✗ $$file has GO_VERSION: $$FILE_VER (expected $$GO_VER)$(COLOR_RESET)"; \
					ERRORS=$$((ERRORS + 1)); \
				else \
					echo "$(COLOR_GREEN)✓ $$file$(COLOR_RESET)"; \
				fi; \
			fi; \
			if grep -q "go-version:" "$$file"; then \
				FILE_VER=$$(grep "go-version:" "$$file" | grep -v "{{" | head -1 | sed "s/.*go-version: '\([^']*\)'.*/\1/"); \
				if [ -n "$$FILE_VER" ] && [ "$$FILE_VER" != "$$GO_VER" ]; then \
					echo "$(COLOR_RED)✗ $$file has go-version: $$FILE_VER (expected $$GO_VER)$(COLOR_RESET)"; \
					ERRORS=$$((ERRORS + 1)); \
				fi; \
			fi; \
		fi; \
	done; \
	GOMOD_VER=$$(sed -n '3p' go.mod | awk '{print $$2}'); \
	if [ "$$GOMOD_VER" != "$$GO_VER" ]; then \
		echo "$(COLOR_RED)✗ go.mod has go $$GOMOD_VER (expected $$GO_VER)$(COLOR_RESET)"; \
		ERRORS=$$((ERRORS + 1)); \
	else \
		echo "$(COLOR_GREEN)✓ go.mod$(COLOR_RESET)"; \
	fi; \
	if [ -f docs/github-workflows.md ]; then \
		DOC_VERS=$$(grep -o "Go Version: \`[^\`]*\`" docs/github-workflows.md | grep -o "[0-9][^\`]*" | sort -u); \
		for DOC_VER in $$DOC_VERS; do \
			if [ "$$DOC_VER" != "$$GO_VER" ]; then \
				echo "$(COLOR_RED)✗ docs/github-workflows.md has Go Version: $$DOC_VER (expected $$GO_VER)$(COLOR_RESET)"; \
				ERRORS=$$((ERRORS + 1)); \
				break; \
			fi; \
		done; \
		if [ $$ERRORS -eq 0 ] || [ -z "$$DOC_VERS" ]; then \
			echo "$(COLOR_GREEN)✓ docs/github-workflows.md$(COLOR_RESET)"; \
		fi; \
	fi; \
	if [ $$ERRORS -eq 0 ]; then \
		echo "$(COLOR_GREEN)All files have consistent Go version $$GO_VER$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_RED)Found $$ERRORS version mismatches. Run 'make sync-go-version' to fix.$(COLOR_RESET)"; \
		exit 1; \
	fi

# =============================================================================
# Development Environment
# =============================================================================

.PHONY: dev
dev: tools ## Set up development environment (mise, deps, git hooks)
	@echo "$(COLOR_YELLOW)Setting up mise...$(COLOR_RESET)"
	@if ! command -v mise >/dev/null 2>&1; then \
		echo "$(COLOR_RED)Warning: mise is not installed$(COLOR_RESET)"; \
		echo "Install mise from: https://mise.jdx.dev/getting-started.html"; \
	else \
		mise trust 2>/dev/null || true; \
		mise install; \
		echo "$(COLOR_GREEN)mise configured$(COLOR_RESET)"; \
	fi
	@$(MAKE) check-go-version
	@$(MAKE) deps
	@echo "$(COLOR_YELLOW)Installing pre-commit hook...$(COLOR_RESET)"
	@echo '#!/bin/bash' > .git/hooks/pre-commit
	@echo 'if command -v mise >/dev/null 2>&1; then' >> .git/hooks/pre-commit
	@echo '  mise exec -- make precommit' >> .git/hooks/pre-commit
	@echo 'else' >> .git/hooks/pre-commit
	@echo '  make precommit' >> .git/hooks/pre-commit
	@echo 'fi' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "$(COLOR_GREEN)Development environment ready$(COLOR_RESET)"

# =============================================================================
# CI/CD
# =============================================================================

.PHONY: check
check: lint vet test-short ## Quick code quality check (lint, vet, short tests)

.PHONY: precommit
precommit: fmt tidy lint test ## Run fast pre-commit checks (fmt, tidy, lint, test)

.PHONY: ci
ci: deps fmt tidy lint vet test test-race test-coverage test-integration build vuln ## Run comprehensive CI pipeline

# =============================================================================
# Cleanup
# =============================================================================

.PHONY: clean
clean: ## Clean build artifacts and caches
	@echo "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	rm -rf $(BUILD_DIR) $(OUTPUT_DIR) $(COVERAGE_DIR)
	$(GO) clean -cache -testcache -modcache
	@echo "$(COLOR_GREEN)Cleanup completed$(COLOR_RESET)"

# =============================================================================
# Changelog
# =============================================================================

.PHONY: changelog
changelog: ## Generate changelog from git history
	@echo "$(COLOR_YELLOW)Generating changelog...$(COLOR_RESET)"
	git cliff -o CHANGELOG.md
	@echo "$(COLOR_GREEN)Changelog generated$(COLOR_RESET)"

.PHONY: changelog-preview
changelog-preview: ## Preview unreleased changelog entries
	@echo "$(COLOR_YELLOW)Previewing unreleased changes...$(COLOR_RESET)"
	git cliff --unreleased

# =============================================================================
# Release Management (GoReleaser workflow)
# =============================================================================

.PHONY: release
release: ## Show release plan (dry-run)
	@if [ -z "$(TYPE)" ]; then \
		echo "Usage: make release TYPE={alpha|beta|rc|stable}"; \
		echo "       make release TYPE=bump SCOPE={minor|major}"; \
		exit 1; \
	fi
	@DRY_RUN=1 ./scripts/semver.sh release $(TYPE) $(SCOPE)

.PHONY: release-do
release-do: ## Execute release (creates tag)
	@if [ -z "$(TYPE)" ]; then \
		echo "Usage: make release-do TYPE={alpha|beta|rc|stable}"; \
		echo "       make release-do TYPE=bump SCOPE={minor|major}"; \
		exit 1; \
	fi
	@./scripts/semver.sh release $(TYPE) $(SCOPE)

# =============================================================================
# Default target
# =============================================================================

.DEFAULT_GOAL := help