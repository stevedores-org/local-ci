.PHONY: build test clean install help

# Build variables
BINARY_NAME=local-ci
VERSION=$(shell git describe --tags --always --dirty)
BUILD_DIR=dist

help:
	@echo "local-ci — Makefile targets"
	@echo ""
	@echo "Targets:"
	@echo "  build       Build the binary"
	@echo "  test        Run Go tests"
	@echo "  install     Install binary locally"
	@echo "  clean       Remove build artifacts"
	@echo "  help        Show this help message"

build:
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) -ldflags="-X main.version=$(VERSION)" .
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

test:
	go test -v ./...

install: build
	@echo "Installing $(BINARY_NAME)..."
	@go install -ldflags="-X main.version=$(VERSION)" .
	@echo "Installed $(BINARY_NAME)"

clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@rm -f .local-ci-cache
	@echo "Clean complete"
