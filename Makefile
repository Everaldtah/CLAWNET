# CLAWNET Makefile
# Build, test, and deployment automation

# Variables
BINARY_NAME=clawnet
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -s -w"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Directories
BUILD_DIR=./build
DIST_DIR=./dist
CMD_DIR=./cmd/clawnet

# Platforms
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build clean test deps lint fmt vet install uninstall docker docker-push help

# Default target
all: deps fmt vet test build

# Build binary for current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d'/' -f1); \
		GOARCH=$$(echo $$platform | cut -d'/' -f2); \
		OUTPUT=$(DIST_DIR)/$(BINARY_NAME)-$$GOOS-$$GOARCH; \
		if [ "$$GOOS" = "windows" ]; then OUTPUT="$$OUTPUT.exe"; fi; \
		echo "Building for $$GOOS/$$GOARCH..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH $(GOBUILD) $(LDFLAGS) -o $$OUTPUT $(CMD_DIR)/main.go; \
	done
	@echo "All builds complete in $(DIST_DIR)/"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"

# Run tests with coverage
test-coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies ready"

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy
	@echo "Dependencies updated"

# Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...
	@echo "Lint complete"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...
	@echo "Format complete"

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...
	@echo "Vet complete"

# Install binary to system
install: build
	@echo "Installing $(BINARY_NAME)..."
	@install -Dm755 $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@install -Dm644 deployments/clawnet.service /etc/systemd/system/clawnet.service
	@mkdir -p /etc/clawnet /var/lib/clawnet
	@install -Dm600 configs/config.example.yaml /etc/clawnet/config.yaml
	@useradd -r -s /bin/false clawnet 2>/dev/null || true
	@chown -R clawnet:clawnet /var/lib/clawnet
	@echo "Installation complete"
	@echo "Run 'systemctl enable --now clawnet' to start the service"

# Uninstall binary from system
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@systemctl stop clawnet 2>/dev/null || true
	@systemctl disable clawnet 2>/dev/null || true
	@rm -f /usr/local/bin/$(BINARY_NAME)
	@rm -f /etc/systemd/system/clawnet.service
	@echo "Uninstall complete"

# Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t clawnet:$(VERSION) -t clawnet:latest .
	@echo "Docker build complete"

# Push Docker image
docker-push: docker
	@echo "Pushing Docker image..."
	docker tag clawnet:$(VERSION) clawnet/clawnet:$(VERSION)
	docker tag clawnet:latest clawnet/clawnet:latest
	docker push clawnet/clawnet:$(VERSION)
	docker push clawnet/clawnet:latest
	@echo "Docker push complete"

# Run with Docker Compose
docker-up:
	@echo "Starting with Docker Compose..."
	docker-compose up -d
	@echo "Services started"

# Stop Docker Compose
docker-down:
	@echo "Stopping Docker Compose..."
	docker-compose down
	@echo "Services stopped"

# View Docker logs
docker-logs:
	docker-compose logs -f clawnet

# Initialize configuration
init:
	@echo "Initializing CLAWNET..."
	@mkdir -p ~/.clawnet
	@cp configs/config.example.yaml ~/.clawnet/config.yaml
	@echo "Configuration initialized at ~/.clawnet/config.yaml"
	@echo "Please edit the configuration file before starting"

# Run the node
run: build
	@echo "Starting CLAWNET node..."
	$(BUILD_DIR)/$(BINARY_NAME)

# Run with specific config
run-config: build
	@echo "Starting CLAWNET node with config..."
	$(BUILD_DIR)/$(BINARY_NAME) --config $(CONFIG)

# Debug build
debug:
	@echo "Building debug version..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)-debug $(CMD_DIR)/main.go
	@echo "Debug build complete"

# Generate protobuf files (if needed)
generate:
	@echo "Generating code..."
	$(GOCMD) generate ./...
	@echo "Generation complete"

# Security scan
security-scan:
	@echo "Running security scan..."
	@which govulncheck > /dev/null || (echo "Installing govulncheck..." && go install golang.org/x/vuln/cmd/govulncheck@latest)
	govulncheck ./...
	@echo "Security scan complete"

# Benchmark
benchmark:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...
	@echo "Benchmarks complete"

# Create release tarball
release: build-all
	@echo "Creating release tarball..."
	@mkdir -p $(DIST_DIR)
	@tar -czf $(DIST_DIR)/$(BINARY_NAME)-$(VERSION).tar.gz -C $(DIST_DIR) $(BINARY_NAME)-*
	@echo "Release created: $(DIST_DIR)/$(BINARY_NAME)-$(VERSION).tar.gz"

# Help
help:
	@echo "CLAWNET Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  make build         - Build binary for current platform"
	@echo "  make build-all     - Build for all platforms"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make test          - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make deps          - Download dependencies"
	@echo "  make lint          - Run linter"
	@echo "  make fmt           - Format code"
	@echo "  make vet           - Run go vet"
	@echo "  make install       - Install to system"
	@echo "  make uninstall     - Uninstall from system"
	@echo "  make docker        - Build Docker image"
	@echo "  make docker-up     - Start with Docker Compose"
	@echo "  make docker-down   - Stop Docker Compose"
	@echo "  make init          - Initialize configuration"
	@echo "  make run           - Build and run"
	@echo "  make debug         - Build debug version"
	@echo "  make release       - Create release tarball"
	@echo "  make help          - Show this help"

# Cross-compilation targets
linux-amd64:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)/main.go

linux-arm64:
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)/main.go

darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)/main.go

darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)/main.go

windows-amd64:
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)/main.go
