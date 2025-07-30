# nerdctl gRPC Server Makefile
# Provides convenient commands for building, testing, and development

# Configuration
GO_VERSION := 1.21
PROJECT_NAME := nerdctl-grpc-server
BUILD_DIR := ./bin
TEST_DIR := ./test
COVERAGE_DIR := ./coverage
PROTO_DIR := ./proto

# Build flags
LDFLAGS := -ldflags "-s -w -X main.version=$(shell git describe --tags --always --dirty) -X main.commit=$(shell git rev-parse HEAD) -X main.buildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"
BUILD_FLAGS := -trimpath $(LDFLAGS)

# Test flags
TEST_FLAGS := -v -race -timeout=10m
COVERAGE_FLAGS := -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic
BENCH_FLAGS := -bench=. -benchmem -benchtime=5s

# Default target
.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Clean build and test artifacts
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -rf $(COVERAGE_DIR)
	@rm -rf ./tmp
	@go clean -testcache
	@go clean -modcache 2>/dev/null || true
	@echo "✅ Clean completed"

##@ Development

.PHONY: deps
deps: ## Download and tidy dependencies
	@echo "📦 Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependencies installed"

.PHONY: generate
generate: ## Generate code from protobuf and mocks
	@echo "🔧 Generating code..."
	@mkdir -p $(PROTO_DIR)/gen
	@go generate ./...
	@echo "✅ Code generation completed"

.PHONY: fmt
fmt: ## Format Go code
	@echo "📝 Formatting code..."
	@go fmt ./...
	@gofmt -s -w .
	@echo "✅ Code formatting completed"

.PHONY: lint
lint: ## Run linters
	@echo "🔍 Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --config .golangci.yml ./...; \
	else \
		echo "⚠️  golangci-lint not installed, skipping linting"; \
		echo "   Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi
	@echo "✅ Linting completed"

.PHONY: vet
vet: ## Run go vet
	@echo "🔍 Running go vet..."
	@go vet ./...
	@echo "✅ Vetting completed"

##@ Building

.PHONY: build
build: deps ## Build the gRPC server
	@echo "🔨 Building $(PROJECT_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME) ./cmd/server
	@echo "✅ Build completed: $(BUILD_DIR)/$(PROJECT_NAME)"

.PHONY: build-debug
build-debug: deps ## Build with debug symbols
	@echo "🔨 Building $(PROJECT_NAME) with debug symbols..."
	@mkdir -p $(BUILD_DIR)
	@go build -gcflags="all=-N -l" -o $(BUILD_DIR)/$(PROJECT_NAME)-debug ./cmd/server
	@echo "✅ Debug build completed: $(BUILD_DIR)/$(PROJECT_NAME)-debug"

.PHONY: install
install: build ## Install binary to GOPATH/bin
	@echo "📦 Installing $(PROJECT_NAME)..."
	@go install $(BUILD_FLAGS) ./cmd/server
	@echo "✅ Installation completed"

##@ Testing

.PHONY: test
test: ## Run unit tests
	@echo "🧪 Running unit tests..."
	@mkdir -p $(COVERAGE_DIR)
	@go test $(TEST_FLAGS) $(COVERAGE_FLAGS) ./...
	@echo "✅ Unit tests completed"

.PHONY: test-unit
test-unit: ## Run unit tests only
	@echo "🧪 Running unit tests..."
	@mkdir -p $(COVERAGE_DIR)
	@go test $(TEST_FLAGS) $(COVERAGE_FLAGS) \
		./internal/... \
		./pkg/... \
		./test/adapters/... \
		./test/mocks/...
	@echo "✅ Unit tests completed"

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "🧪 Running integration tests..."
	@echo "⚠️  Integration tests require nerdctl binaries"
	@NERDCTL_INTEGRATION_TESTS=1 go test $(TEST_FLAGS) -timeout=15m ./test/integration/...
	@echo "✅ Integration tests completed"

.PHONY: test-stress
test-stress: ## Run stress tests
	@echo "🧪 Running stress tests..."
	@echo "⚠️  Stress tests may take a long time"
	@go test $(TEST_FLAGS) -timeout=20m ./test/stress/...
	@echo "✅ Stress tests completed"

.PHONY: test-all
test-all: test-unit test-integration test-stress ## Run all tests
	@echo "✅ All tests completed"

.PHONY: test-quick
test-quick: ## Run quick tests (no race detection, no coverage)
	@echo "🧪 Running quick tests..."
	@go test -v -timeout=5m ./internal/... ./pkg/...
	@echo "✅ Quick tests completed"

.PHONY: test-race
test-race: ## Run tests with race detection
	@echo "🧪 Running tests with race detection..."
	@go test -race -timeout=15m ./...
	@echo "✅ Race tests completed"

.PHONY: test-cover
test-cover: test ## Generate and display coverage report
	@echo "📊 Generating coverage report..."
	@go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@go tool cover -func=$(COVERAGE_DIR)/coverage.out
	@echo "📊 Coverage report generated: $(COVERAGE_DIR)/coverage.html"

.PHONY: bench
bench: ## Run benchmarks
	@echo "🚀 Running benchmarks..."
	@mkdir -p $(COVERAGE_DIR)
	@go test $(BENCH_FLAGS) -run=^$$ ./... | tee $(COVERAGE_DIR)/benchmark.txt
	@echo "✅ Benchmarks completed"

##@ Test Environment

.PHONY: test-env-setup
test-env-setup: ## Set up test environment
	@echo "🔧 Setting up test environment..."
	@mkdir -p ./tmp/nerdctl-v1 ./tmp/nerdctl-v2
	@echo "⚠️  Please manually place nerdctl binaries in ./tmp/ directories"
	@echo "   - ./tmp/nerdctl-v1/nerdctl (for v1.x testing)"
	@echo "   - ./tmp/nerdctl-v2/nerdctl (for v2.x testing)"
	@echo "✅ Test environment directories created"

.PHONY: test-env-check
test-env-check: ## Check test environment
	@echo "🔍 Checking test environment..."
	@go run test/test_runner.go validate
	@echo "✅ Test environment check completed"

.PHONY: test-with-runner
test-with-runner: ## Run tests using the test runner
	@echo "🧪 Running tests with test runner..."
	@go run test/test_runner.go run all
	@echo "✅ Test runner completed"

##@ Quality Assurance

.PHONY: qa
qa: deps fmt vet lint test-unit ## Run full quality assurance pipeline
	@echo "✅ Quality assurance pipeline completed"

.PHONY: qa-full
qa-full: deps fmt vet lint test-all bench ## Run comprehensive quality assurance
	@echo "✅ Comprehensive quality assurance completed"

.PHONY: security-scan
security-scan: ## Run security scanning
	@echo "🔒 Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "⚠️  gosec not installed, skipping security scan"; \
		echo "   Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi
	@echo "✅ Security scan completed"

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "🐳 Building Docker image..."
	@docker build -t $(PROJECT_NAME):latest .
	@docker build -t $(PROJECT_NAME):$(shell git describe --tags --always) .
	@echo "✅ Docker image built"

.PHONY: docker-test
docker-test: docker-build ## Run tests in Docker container
	@echo "🐳 Running tests in Docker..."
	@docker run --rm $(PROJECT_NAME):latest make test-unit
	@echo "✅ Docker tests completed"

##@ Development Workflow

.PHONY: dev-setup
dev-setup: deps generate ## Set up development environment
	@echo "🔧 Setting up development environment..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "✅ Development environment setup completed"

.PHONY: dev-test
dev-test: ## Run development tests (quick feedback loop)
	@echo "🧪 Running development tests..."
	@go test -short -timeout=5m ./internal/... ./pkg/...
	@echo "✅ Development tests completed"

.PHONY: pre-commit
pre-commit: fmt vet lint test-quick ## Run pre-commit checks
	@echo "✅ Pre-commit checks completed"

.PHONY: pre-push
pre-push: qa ## Run pre-push checks
	@echo "✅ Pre-push checks completed"

##@ CI/CD

.PHONY: ci-test
ci-test: ## Run CI tests
	@echo "🤖 Running CI tests..."
	@CI=1 go test $(TEST_FLAGS) $(COVERAGE_FLAGS) ./...
	@echo "✅ CI tests completed"

.PHONY: ci-build
ci-build: ## Build for CI
	@echo "🤖 Building for CI..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=linux go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME)-linux ./cmd/server
	@CGO_ENABLED=0 GOOS=darwin go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME)-darwin ./cmd/server
	@CGO_ENABLED=0 GOOS=windows go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME)-windows.exe ./cmd/server
	@echo "✅ CI build completed"

##@ Maintenance

.PHONY: mod-update
mod-update: ## Update Go modules
	@echo "📦 Updating Go modules..."
	@go get -u ./...
	@go mod tidy
	@echo "✅ Modules updated"

.PHONY: mod-verify
mod-verify: ## Verify Go modules
	@echo "🔍 Verifying Go modules..."
	@go mod verify
	@echo "✅ Modules verified"

##@ Documentation

.PHONY: docs
docs: ## Generate documentation
	@echo "📚 Generating documentation..."
	@mkdir -p ./docs
	@go doc -all . > ./docs/api.md
	@echo "✅ Documentation generated"

.PHONY: docs-serve
docs-serve: ## Serve documentation locally
	@echo "📚 Serving documentation..."
	@if command -v godoc >/dev/null 2>&1; then \
		echo "📖 Documentation available at http://localhost:6060/pkg/$(shell go mod why | head -n1 | cut -d' ' -f2)/"; \
		godoc -http=:6060; \
	else \
		echo "⚠️  godoc not installed"; \
		echo "   Install with: go install golang.org/x/tools/cmd/godoc@latest"; \
	fi

# Make sure intermediate files are not deleted
.PRECIOUS: $(COVERAGE_DIR)/coverage.out

# Phony targets (targets that don't create files)
.PHONY: help clean deps generate fmt lint vet build build-debug install \
	test test-unit test-integration test-stress test-all test-quick test-race test-cover bench \
	test-env-setup test-env-check test-with-runner \
	qa qa-full security-scan \
	docker-build docker-test \
	dev-setup dev-test pre-commit pre-push \
	ci-test ci-build \
	mod-update mod-verify \
	docs docs-serve