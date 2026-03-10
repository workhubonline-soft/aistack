# AIStack Makefile
# Usage:
#   make build          — build for current platform
#   make build-all      — cross-compile all platforms
#   make test           — run tests
#   make lint           — run linter
#   make release        — full release build (all platforms + checksums)
#   make docker-build   — build Docker image for CLI
#   make clean          — remove build artifacts
#   make dev            — run CLI locally (go run)

# ── Variables ─────────────────────────────────────────────────────────────────
BINARY_NAME    := aistack
MODULE         := github.com/workhubonline-soft/aistack
CLI_DIR        := ./cli
BUILD_DIR      := ./dist
INSTALL_PATH   := /usr/local/bin/aistack

# Version from git tag, fallback to dev
VERSION        := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT         := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE     := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go build flags
LDFLAGS := -s -w \
	-X 'github.com/workhubonline-soft/aistack/cmd.version=$(VERSION)' \
	-X 'github.com/workhubonline-soft/aistack/cmd.commit=$(COMMIT)' \
	-X 'github.com/workhubonline-soft/aistack/cmd.buildDate=$(BUILD_DATE)'

GOFLAGS := -trimpath

# Platforms for cross-compilation
PLATFORMS := \
	linux/amd64 \
	linux/arm64

# Colors
GREEN  := \033[0;32m
YELLOW := \033[1;33m
NC     := \033[0m

# ── Help ──────────────────────────────────────────────────────────────────────
.PHONY: help
help: ## Show this help
	@echo ""
	@echo "  $(YELLOW)AIStack Build System$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""

# ── Dependencies ──────────────────────────────────────────────────────────────
.PHONY: deps
deps: ## Download and tidy Go dependencies
	@echo "→ Downloading dependencies..."
	@cd $(CLI_DIR) && go mod download
	@cd $(CLI_DIR) && go mod tidy
	@echo "$(GREEN)✓ Dependencies ready$(NC)"

.PHONY: deps-update
deps-update: ## Update all dependencies to latest
	@cd $(CLI_DIR) && go get -u ./...
	@cd $(CLI_DIR) && go mod tidy

# ── Build ─────────────────────────────────────────────────────────────────────
.PHONY: build
build: deps ## Build for current platform
	@echo "→ Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	@cd $(CLI_DIR) && go build $(GOFLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/$(BINARY_NAME) ./...
	@echo "$(GREEN)✓ Built: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

.PHONY: build-all
build-all: deps ## Cross-compile for all platforms
	@echo "→ Cross-compiling for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@$(foreach PLATFORM,$(PLATFORMS),\
		$(eval OS=$(word 1,$(subst /, ,$(PLATFORM)))) \
		$(eval ARCH=$(word 2,$(subst /, ,$(PLATFORM)))) \
		echo "  Building $(OS)/$(ARCH)..." && \
		cd $(CLI_DIR) && \
		CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) \
		go build $(GOFLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/$(BINARY_NAME)-$(OS)-$(ARCH) ./... && \
		echo "$(GREEN)  ✓ $(BUILD_DIR)/$(BINARY_NAME)-$(OS)-$(ARCH)$(NC)" ; \
	)
	@echo "$(GREEN)✓ All builds complete$(NC)"

.PHONY: build-linux-amd64
build-linux-amd64: ## Build linux/amd64 specifically
	@mkdir -p $(BUILD_DIR)
	@cd $(CLI_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build $(GOFLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./...
	@echo "$(GREEN)✓ $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64$(NC)"

.PHONY: build-linux-arm64
build-linux-arm64: ## Build linux/arm64 (Raspberry Pi 5, ARM servers)
	@mkdir -p $(BUILD_DIR)
	@cd $(CLI_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build $(GOFLAGS) -ldflags="$(LDFLAGS)" \
		-o ../$(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./...
	@echo "$(GREEN)✓ $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64$(NC)"

# ── Release ───────────────────────────────────────────────────────────────────
.PHONY: release
release: clean build-all checksums ## Full release build (all platforms + checksums)
	@echo "$(GREEN)✓ Release artifacts in $(BUILD_DIR)/$(NC)"
	@ls -lh $(BUILD_DIR)/

.PHONY: checksums
checksums: ## Generate SHA256 checksums for all binaries
	@echo "→ Generating checksums..."
	@cd $(BUILD_DIR) && sha256sum $(BINARY_NAME)-* > checksums.txt
	@echo "$(GREEN)✓ Checksums: $(BUILD_DIR)/checksums.txt$(NC)"
	@cat $(BUILD_DIR)/checksums.txt

# ── Test ──────────────────────────────────────────────────────────────────────
.PHONY: test
test: ## Run all tests
	@echo "→ Running tests..."
	@cd $(CLI_DIR) && go test ./... -v -race -timeout 60s
	@echo "$(GREEN)✓ All tests passed$(NC)"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@mkdir -p $(BUILD_DIR)
	@cd $(CLI_DIR) && go test ./... \
		-coverprofile=../$(BUILD_DIR)/coverage.out \
		-covermode=atomic
	@cd $(CLI_DIR) && go tool cover \
		-html=../$(BUILD_DIR)/coverage.out \
		-o ../$(BUILD_DIR)/coverage.html
	@echo "$(GREEN)✓ Coverage: $(BUILD_DIR)/coverage.html$(NC)"

.PHONY: test-short
test-short: ## Run tests without integration tests
	@cd $(CLI_DIR) && go test ./... -short -timeout 30s

# ── Lint ──────────────────────────────────────────────────────────────────────
.PHONY: lint
lint: ## Run golangci-lint
	@which golangci-lint > /dev/null 2>&1 || \
		(echo "Installing golangci-lint..." && \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
		sh -s -- -b $$(go env GOPATH)/bin v1.57.2)
	@cd $(CLI_DIR) && golangci-lint run ./...
	@echo "$(GREEN)✓ Lint passed$(NC)"

.PHONY: fmt
fmt: ## Format Go code
	@cd $(CLI_DIR) && gofmt -s -w .
	@cd $(CLI_DIR) && goimports -w . 2>/dev/null || true
	@echo "$(GREEN)✓ Code formatted$(NC)"

.PHONY: vet
vet: ## Run go vet
	@cd $(CLI_DIR) && go vet ./...
	@echo "$(GREEN)✓ Vet passed$(NC)"

# ── Install ───────────────────────────────────────────────────────────────────
.PHONY: install
install: build ## Install aistack binary to system
	@echo "→ Installing to $(INSTALL_PATH)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)
	@sudo chmod 755 $(INSTALL_PATH)
	@echo "$(GREEN)✓ Installed: $(INSTALL_PATH)$(NC)"
	@$(BINARY_NAME) version

.PHONY: uninstall
uninstall: ## Remove installed binary
	@sudo rm -f $(INSTALL_PATH)
	@echo "$(GREEN)✓ Uninstalled$(NC)"

# ── Docker ────────────────────────────────────────────────────────────────────
.PHONY: docker-build
docker-build: ## Build Docker image for CLI
	@docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		-t aistack:$(VERSION) \
		-t aistack:latest \
		-f Dockerfile .
	@echo "$(GREEN)✓ Docker image: aistack:$(VERSION)$(NC)"

.PHONY: docker-push
docker-push: docker-build ## Push Docker image to registry
	@docker push ghcr.io/workhubonline-soft/aistack:$(VERSION)
	@docker push ghcr.io/workhubonline-soft/aistack:latest

# ── Dev ───────────────────────────────────────────────────────────────────────
.PHONY: dev
dev: ## Run CLI directly with go run
	@cd $(CLI_DIR) && go run . $(ARGS)

.PHONY: dev-doctor
dev-doctor: ## Quick: run doctor command
	@cd $(CLI_DIR) && go run . doctor

.PHONY: dev-models
dev-models: ## Quick: run models list
	@cd $(CLI_DIR) && go run . models list

.PHONY: watch
watch: ## Watch for changes and rebuild (requires entr)
	@which entr > /dev/null 2>&1 || (echo "Install entr: apt install entr" && exit 1)
	@find $(CLI_DIR) -name '*.go' | entr -r make dev ARGS="$(ARGS)"

# ── Catalog ───────────────────────────────────────────────────────────────────
.PHONY: validate-catalog
validate-catalog: ## Validate models/catalog.yaml syntax
	@python3 -c "import yaml; yaml.safe_load(open('models/catalog.yaml'))" && \
		echo "$(GREEN)✓ Catalog YAML valid$(NC)" || \
		echo "✗ Catalog YAML invalid"

# ── Clean ─────────────────────────────────────────────────────────────────────
.PHONY: clean
clean: ## Remove build artifacts
	@rm -rf $(BUILD_DIR)
	@echo "$(GREEN)✓ Cleaned$(NC)"

# ── Version ───────────────────────────────────────────────────────────────────
.PHONY: version
version: ## Show current version info
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build date: $(BUILD_DATE)"

# ── CI shortcut ───────────────────────────────────────────────────────────────
.PHONY: ci
ci: deps vet test build-all checksums ## Full CI pipeline (deps + vet + test + build)
	@echo "$(GREEN)✓ CI pipeline complete$(NC)"

.DEFAULT_GOAL := help
