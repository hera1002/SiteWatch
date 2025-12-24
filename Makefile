.PHONY: build run test clean install help

# Binary name
BINARY_NAME=cronzee
CONFIG_FILE=config.yaml

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) .
	@echo "Build complete: ./$(BINARY_NAME)"

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BINARY_NAME) -config $(CONFIG_FILE)

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@go clean
	@echo "Clean complete"

# Install dependencies
install:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies installed"

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 .
	@GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin-amd64 .
	@GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 .
	@GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-amd64.exe .
	@echo "Multi-platform build complete"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete"

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run || echo "golangci-lint not installed, skipping..."

# Show help
help:
	@echo "Available targets:"
	@echo "  build      - Build the application"
	@echo "  run        - Build and run the application"
	@echo "  test       - Run tests"
	@echo "  clean      - Remove build artifacts"
	@echo "  install    - Install dependencies"
	@echo "  build-all  - Build for multiple platforms"
	@echo "  fmt        - Format code"
	@echo "  lint       - Run linter"
	@echo "  help       - Show this help message"
