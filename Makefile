# Go 
GO111MODULE     := on
CGO_ENABLED	    := 0

# Environment for Go commands
GO_ENV := \
	GO111MODULE=$(GO111MODULE) \
	CGO_ENABLED=$(CGO_ENABLED)

# Default target
all: help

.PHONY: help
help: ## Displays available commands
	@echo "Available make targets:"; \
	grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk \
		'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Test targets
.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	$(GO_ENV) go test -count=1 -race -covermode=atomic -coverprofile=coverage.out ./...
	$(GO_ENV) go tool cover -func=coverage.out

.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GO_ENV) go test ./pkg/... -bench=. -benchmem

# Code quality targets
.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting code..."
	$(GO_ENV) go fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	$(GO_ENV) go vet ./...

.PHONY: lint
lint: lint-go lint-yaml ## Lints the entire project
	@echo "Completed Go and YAML lints"

.PHONY: lint-go
lint-go: ## Lints the Go files
	@echo "Running golangci-lint..."
	$(GO_ENV) golangci-lint -c .golangci.yaml run

.PHONY: lint-yaml
lint-yaml: ## Lints YAML files
	@if [ -n "$(YAML_FILES)" ]; then \
		yamllint -c .yamllint $(YAML_FILES); \
	else \
		echo "No YAML files found to lint."; \
	fi

# Dependency management
.PHONY: deps
deps: ## Download and tidy dependencies
	@echo "Managing dependencies..."
	$(GO_ENV) go mod download
	$(GO_ENV) go mod tidy

.PHONY: deps-update
deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	$(GO_ENV) go get -u ./...
	$(GO_ENV) go mod tidy

# Cleanup
.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	$(GO_ENV) go clean

# CI/CD targets
.PHONY: ci
ci: fmt vet lint test ## Run all CI checks
	@echo "All CI checks passed!"

# Release 
.PHONY: bump-major
bump-major: ## Bumps major version (1.2.3 → 2.0.0)
	@echo "Bumping major version..."
	tools/bump major

.PHONY: bump-minor
bump-minor: ## Bumps minor version (1.2.3 → 1.3.0)
	@echo "Bumping minor version..."
	tools/bump minor

.PHONY: bump-patch
bump-patch: ## Bumps patch version (1.2.3 → 1.2.4)
	@echo "Bumping patch version..."
	tools/bump patch

.PHONY: release
release: clean ## Runs the release process
	@echo "Releasing new version..."
	GITLAB_TOKEN="" goreleaser release --clean --config .goreleaser.yaml --fail-fast --timeout 10m0s
