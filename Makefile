.PHONY: proto build test lint clean install build-darwin build-linux build-windows release

# Default target
all: test build

# Generate protobuf code
proto:
	protoc --go_out=. --go_opt=paths=source_relative proto/dmgn/v1/dmgn.proto

# Build the binary
build:
	CGO_ENABLED=1 go build -ldflags="-s -w" -o dmgn ./cmd/dmgn

# Build for Linux
build-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dmgn-linux-amd64 ./cmd/dmgn

# Build for macOS
build-darwin:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dmgn-darwin-amd64 ./cmd/dmgn
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dmgn-darwin-arm64 ./cmd/dmgn

# Build for Windows
build-windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dmgn-windows-amd64.exe ./cmd/dmgn

# Build all platforms
build-all: build-linux build-darwin build-windows

# Run tests
test:
	go test -v -race -coverprofile=coverage.out ./...

# Run tests with verbose output
test-verbose:
	go test -v -race ./...

# Run linter
lint:
	golangci-lint run ./...

# Install dependencies
install:
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Clean build artifacts
clean:
	rm -f dmgn*
	rm -f coverage.out

# Run security check
security:
	govulncheck ./...

# Format code
fmt:
	gofmt -w -s .
	goimports -w -s .

# Generate code (proto, docs, etc.)
generate:
	go generate ./...

# Build release binaries
release:
	@echo "Creating release builds..."
	@mkdir -p release
	@CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=$(git describe --tags)" -o release/dmgn-linux-amd64 ./cmd/dmgn
	@CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.version=$(git describe --tags)" -o release/dmgn-darwin-amd64 ./cmd/dmgn
	@CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.version=$(git describe --tags)" -o release/dmgn-darwin-arm64 ./cmd/dmgn
	@CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.version=$(git describe --tags)" -o release/dmgn-windows-amd64.exe ./cmd/dmgn
	@echo "Release builds created in release/ directory"
