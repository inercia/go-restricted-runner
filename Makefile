.PHONY: test lint lint-golangci format clean help

# Go related variables
GOBASE=$(shell pwd)

# Default target
all: test

# Run tests
test:
	@echo ">>> Running tests..."
	@go test -v ./...
	@echo ">>> ... tests completed successfully"

# Run tests with race detection
test-race:
	@echo ">>> Running tests with race detection..."
	@go test -race -v ./...
	@echo ">>> ... tests completed successfully"

# Run tests with coverage
test-coverage:
	@echo ">>> Running tests with coverage..."
	@go test -v -coverprofile=coverage.txt -covermode=atomic ./...
	@echo ">>> ... tests completed successfully"

# Run linting (golangci-lint)
lint: lint-golangci

# Run golangci-lint (comprehensive linting)
lint-golangci:
	@echo ">>> Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run; \
	fi
	@echo ">>> ... golangci-lint completed successfully"

# Format code
format:
	@echo ">>> Formatting Go code..."
	@go fmt ./...
	@go mod tidy
	@echo ">>> ... code formatted successfully"

# Clean build artifacts
clean:
	@echo ">>> Cleaning..."
	@rm -f coverage.txt
	@go clean -cache -testcache

# Verify module dependencies
verify:
	@echo ">>> Verifying dependencies..."
	@go mod verify
	@echo ">>> ... dependencies verified successfully"

# Download dependencies
deps:
	@echo ">>> Downloading dependencies..."
	@go mod download
	@echo ">>> ... dependencies downloaded successfully"

# Show help
help:
	@echo "Available targets:"
	@echo "  test           - Run tests"
	@echo "  test-race      - Run tests with race detection"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  lint           - Run linting (alias for lint-golangci)"
	@echo "  lint-golangci  - Run golangci-lint (installs if not present)"
	@echo "  format         - Format Go code"
	@echo "  clean          - Remove build artifacts"
	@echo "  verify         - Verify module dependencies"
	@echo "  deps           - Download dependencies"
	@echo "  help           - Show this help"

