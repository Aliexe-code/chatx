.PHONY: build run test clean race lint help

# Build the application
build:
	@echo "Building server..."
	@go build -o websocket-server ./cmd/server
	@echo "Build complete: ./websocket-server"

# Run the server
run: build
	@echo "Starting server..."
	@./websocket-server

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run short tests
test-short:
	@echo "Running short tests..."
	@go test -short -v ./...

# Run tests with race detector (with timeout)
test-race:
	@echo "Running tests with race detector..."
	@timeout 60 go test -race -short -v ./... || (echo "Tests timed out or failed" && exit 1)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f websocket-server
	@rm -f websocket-server-race
	@go clean
	@echo "Clean complete"

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run ./... || echo "golangci-lint not installed, skipping..."

# Build with race detector
build-race:
	@echo "Building with race detector..."
	@go build -race -o websocket-server-race ./cmd/server
	@echo "Build complete: ./websocket-server-race"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

# Help
help:
	@echo "Available commands:"
	@echo "  make build       - Build the application"
	@echo "  make run         - Build and run the server"
	@echo "  make test        - Run all tests"
	@echo "  make test-short  - Run short tests"
	@echo "  make test-race   - Run tests with race detector (60s timeout)"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make lint        - Run linter"
	@echo "  make build-race  - Build with race detector"
	@echo "  make fmt         - Format code"
	@echo "  make vet         - Run go vet"
	@echo "  make help        - Show this help message"
