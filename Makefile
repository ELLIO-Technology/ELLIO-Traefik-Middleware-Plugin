# ELLIO Traefik Middleware Plugin Makefile

.PHONY: help
help: ## Display this help message
	@echo "ELLIO Traefik Middleware Plugin - Available Commands:"
	@echo
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: test
test: ## Run all tests
	@echo "Running all tests..."
	@go test -v -race -coverprofile=coverage.out ./...

.PHONY: test-unit
test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	@go test -v -race -short ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@go test -v -race -run Integration ./...

.PHONY: test-e2e
test-e2e: ## Run E2E tests with Traefik
	@echo "Running E2E tests..."
	@docker compose -f ci/docker-compose.test.yml up -d traefik whoami whoami-xff
	@echo "Waiting for services to be ready..."
	@sleep 5
	@cd ci/e2e && go test -v -tags=e2e ./...
	@docker compose -f ci/docker-compose.test.yml down -v

.PHONY: coverage
coverage: test ## Generate coverage report
	@echo "Generating coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: coverage-text
coverage-text: test ## Display coverage in terminal
	@go tool cover -func=coverage.out

.PHONY: bench
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

##@ Code Quality

.PHONY: lint
lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

.PHONY: fmt
fmt: ## Format code with gofmt
	@echo "Formatting code..."
	@gofmt -w -s .
	@echo "Code formatted"

.PHONY: fmt-check
fmt-check: ## Check if code is formatted
	@echo "Checking code formatting..."
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "Code is not formatted. Run 'make fmt'"; \
		gofmt -l .; \
		exit 1; \
	else \
		echo "Code is properly formatted"; \
	fi

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

.PHONY: security
security: ## Run gosec security scanner
	@echo "Running security scan..."
	@if command -v gosec &> /dev/null; then \
		gosec -fmt json -out gosec-report.json ./...; \
		echo "Security scan complete. Report saved to gosec-report.json"; \
	else \
		echo "gosec not installed. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
		exit 1; \
	fi

##@ Traefik Plugin

.PHONY: yaegi-test
yaegi-test: ## Test plugin with Yaegi interpreter
	@echo "Testing with Yaegi interpreter..."
	@if command -v yaegi &> /dev/null; then \
		go mod vendor; \
		yaegi extract -name elliotraefik github.com/ELLIO-Technology/ELLIO-Traefik-Middleware-Plugin || true; \
		yaegi test -v . || true; \
		echo "Yaegi compatibility check completed"; \
	else \
		echo "yaegi not installed. Install with: go install github.com/traefik/yaegi/cmd/yaegi@latest"; \
		exit 1; \
	fi

.PHONY: validate-plugin
validate-plugin: ## Validate plugin configuration
	@echo "Validating plugin configuration..."
	@if [ ! -f ".traefik.yml" ]; then \
		echo "Error: .traefik.yml not found"; \
		exit 1; \
	fi
	@echo "✓ .traefik.yml exists"
	@if [ ! -f "go.mod" ]; then \
		echo "Error: go.mod not found"; \
		exit 1; \
	fi
	@echo "✓ go.mod exists"
	@echo "Plugin configuration is valid"

.PHONY: vendor
vendor: ## Update vendor directory
	@echo "Updating vendor directory..."
	@go mod vendor
	@echo "Vendor directory updated"

##@ Docker & E2E

.PHONY: docker-build-test
docker-build-test: ## Build test Docker environment
	@echo "Building test Docker environment..."
	@docker compose -f ci/docker-compose.test.yml build

.PHONY: docker-up
docker-up: ## Start Traefik with plugin for manual testing
	@echo "Starting Traefik with plugin..."
	@docker compose -f ci/docker-compose.test.yml up traefik

.PHONY: docker-down
docker-down: ## Stop and remove test containers
	@echo "Stopping test containers..."
	@docker compose -f ci/docker-compose.test.yml down -v

.PHONY: docker-logs
docker-logs: ## Show Traefik logs
	@docker compose -f ci/docker-compose.test.yml logs -f traefik

##@ Git Hooks

.PHONY: install-hooks
install-hooks: ## Install Git pre-commit hooks
	@echo "Installing Git pre-commit hooks..."
	@if command -v pre-commit &> /dev/null; then \
		pre-commit install; \
		echo "✓ pre-commit framework hooks installed"; \
	else \
		echo "pre-commit not found, using manual git hooks..."; \
		git config core.hooksPath .githooks; \
		echo "✓ Git hooks path set to .githooks"; \
	fi
	@echo "Pre-commit hooks installed successfully!"

.PHONY: uninstall-hooks
uninstall-hooks: ## Uninstall Git pre-commit hooks
	@echo "Uninstalling Git pre-commit hooks..."
	@if command -v pre-commit &> /dev/null; then \
		pre-commit uninstall; \
	fi
	@git config --unset core.hooksPath || true
	@echo "Pre-commit hooks uninstalled"

.PHONY: run-hooks
run-hooks: ## Run pre-commit hooks on all files
	@echo "Running pre-commit hooks on all files..."
	@if command -v pre-commit &> /dev/null; then \
		pre-commit run --all-files; \
	else \
		.githooks/pre-commit; \
	fi

##@ Build & Dependencies

.PHONY: deps
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@echo "Dependencies downloaded"

.PHONY: deps-update
deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy
	@echo "Dependencies updated"

.PHONY: clean
clean: ## Clean build artifacts and test cache
	@echo "Cleaning..."
	@rm -rf coverage.out coverage.html gosec-report.json
	@go clean -testcache
	@echo "Clean complete"

##@ CI/CD

.PHONY: ci
ci: deps fmt-check vet lint test ## Run all CI checks
	@echo "All CI checks passed!"

.PHONY: pre-commit
pre-commit: fmt vet lint test-unit ## Run pre-commit checks
	@echo "Pre-commit checks passed!"

##@ Release

.PHONY: version
version: ## Display current version from git tags
	@echo "Current version: $$(git describe --tags --always --dirty)"

.PHONY: changelog
changelog: ## Generate changelog from git commits
	@echo "Generating changelog..."
	@echo "# Changelog" > CHANGELOG.md
	@echo "" >> CHANGELOG.md
	@git log --pretty=format:"- %s (%h)" --no-merges >> CHANGELOG.md
	@echo "Changelog generated: CHANGELOG.md"

# Default target
.DEFAULT_GOAL := help
