# Makefile for kubechronicle development

.PHONY: help build run test clean deps fmt vet lint

# Default target
help:
	@echo "kubechronicle Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  deps        - Download and install dependencies"
	@echo "  build       - Build the webhook binary"
	@echo "  run         - Run the webhook locally (requires TLS certs)"
	@echo "  test        - Run tests"
	@echo "  fmt         - Format code"
	@echo "  vet         - Run go vet"
	@echo "  lint        - Run linter (if installed)"
	@echo "  clean       - Clean build artifacts"
	@echo "  docker-build - Build Docker image"

# Download and install dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "✓ Dependencies installed"

# Build the webhook binary
build:
	@echo "Building webhook..."
	@mkdir -p bin
	go build -o bin/webhook ./cmd/webhook
	@echo "✓ Binary built: bin/webhook"

# Run the webhook locally
run: build
	@echo "Running webhook locally..."
	@echo "Note: Requires TLS certificates in ./certs/ directory"
	@if [ ! -f ./certs/tls.crt ] || [ ! -f ./certs/tls.key ]; then \
		echo "Error: TLS certificates not found in ./certs/"; \
		echo "Generate them with: cd deploy/webhook && ./generate-certs.sh"; \
		echo "Then copy tls.crt and tls.key to ./certs/"; \
		exit 1; \
	fi
	./bin/webhook \
		-port=8443 \
		-cert=./certs/tls.crt \
		-key=./certs/tls.key

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out
	@echo ""
	@echo "Coverage report saved to coverage.out"
	@echo "View HTML report with: make coverage-html"
	@echo ""
	@echo "Core packages coverage:"
	@go tool cover -func=coverage.out | grep -E "(admission|diff|config)" | grep total || go tool cover -func=coverage.out | grep total

# Run tests and check coverage threshold (90% average for core packages)
# Core packages: admission, diff, config (excludes cmd/webhook and store)
test-coverage-check:
	@echo "Running tests with coverage check (target: 90% average for core packages)..."
	@go test -v -coverprofile=coverage.out ./internal/admission ./internal/diff ./internal/config
	@echo ""
	@echo "Coverage by package:"
	@go tool cover -func=coverage.out | grep -E "(admission|diff|config)" | grep -v "100.0%" || true
	@echo ""
	@echo "Checking coverage threshold..."
	@total_cov=$$(go tool cover -func=coverage.out 2>/dev/null | grep "total:" | awk '{print $$3}' | sed 's/%//'); \
	if [ -n "$$total_cov" ]; then \
		total_int=$$(echo $$total_cov | cut -d. -f1); \
		echo "Total coverage for core packages: $$total_cov%"; \
		if [ $$total_int -lt 90 ]; then \
			echo "ERROR: Coverage $$total_cov% is below 90% threshold"; \
			echo "Note: cmd/webhook (main) and store (requires DB) are excluded"; \
			exit 1; \
		else \
			echo "✓ Coverage $$total_cov% meets 90% threshold"; \
		fi \
	else \
		echo "✓ All tests passed"; \
	fi

# Generate HTML coverage report
coverage-html:
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report saved to coverage.html"
	@echo "Open with: open coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Run linter (requires golangci-lint)
lint:
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi
	@echo "Running linter..."
	golangci-lint run

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	go clean
	@echo "✓ Cleaned"

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t kubechronicle/webhook:latest .
