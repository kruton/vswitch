# Virtual Switch for QEMU VMs - Makefile

.PHONY: all build lint security test test-unit test-coverage test-coverage-html clean install docker-build docker-run docker-run-daemon docker-stop docker-clean docker-shell

# Build configuration
BINARY_NAME := vswitch
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR := ./bin
GO_FILES := $(shell find . -name '*.go' -type f)
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html

# Build flags
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

# Docker configuration
DOCKER_IMAGE := vswitch
DOCKER_TAG := $(VERSION)
DOCKER_PORTS := -p 9999:9999 -p 9998:9998

# Default target
all: build

# Build the application
build: $(BUILD_DIR)/$(BINARY_NAME)

$(BUILD_DIR)/$(BINARY_NAME): $(GO_FILES)
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Built $(BINARY_NAME) $(VERSION)"

# Run linting
lint:
	@if command -v revive >/dev/null 2>&1; then \
		revive ./...; \
	elif [ -x "$$(go env GOPATH)/bin/revive" ]; then \
		$$(go env GOPATH)/bin/revive ./...; \
	else \
		echo "Warning: revive not found. Install with:"; \
		echo "  go install github.com/mgechev/revive@latest"; \
		echo "Skipping lint checks..."; \
	fi

# Run security checks
security:
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	elif [ -x "$$(go env GOPATH)/bin/gosec" ]; then \
		$$(go env GOPATH)/bin/gosec ./...; \
	else \
		echo "Warning: gosec not found. Install with:"; \
		echo "  go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
		echo "Skipping security checks..."; \
	fi

# Run all tests including lint and security
test: lint security
	go test -v ./switch

# Run only unit tests
test-unit:
	go test -v ./switch

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=$(COVERAGE_FILE) ./switch
	go tool cover -func=$(COVERAGE_FILE)

# Generate HTML coverage report
test-coverage-html: test-coverage
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)
	rm -f $(COVERAGE_FILE)
	rm -f $(COVERAGE_HTML)
	go clean

# Install to local bin (optional)
install: build
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installed $(BINARY_NAME) to /usr/local/bin/"

# Development build (current directory)
dev:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

# Docker targets
docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	@echo "Built Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)"

docker-run:
	@echo "Starting vswitch container on ports 9999 and 9998"
	docker run --rm -it $(DOCKER_PORTS) --name vswitch $(DOCKER_IMAGE):latest

docker-run-daemon:
	@echo "Starting vswitch container in daemon mode"
	docker run -d --restart unless-stopped $(DOCKER_PORTS) --name vswitch-daemon $(DOCKER_IMAGE):latest vswitch -daemon -ports 9999,9998

docker-stop:
	docker stop vswitch-daemon || true
	docker rm vswitch-daemon || true

docker-clean:
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest 2>/dev/null || true
	@echo "Cleaned Docker images"

docker-shell:
	docker run --rm -it --entrypoint /bin/sh $(DOCKER_IMAGE):latest

# Show help
help:
	@echo "Virtual Switch for QEMU VMs - Build System"
	@echo ""
	@echo "Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  build               - Build the application"
	@echo "  dev                 - Build in current directory"
	@echo "  install             - Install to /usr/local/bin"
	@echo "  clean               - Clean build artifacts and coverage files"
	@echo ""
	@echo "Test targets:"
	@echo "  test                - Run all checks (lint, security, unit tests)"
	@echo "  test-unit           - Run only unit tests"
	@echo "  test-coverage       - Run unit tests with coverage report"
	@echo "  test-coverage-html  - Generate HTML coverage report"
	@echo "  lint                - Run revive linter checks"
	@echo "  security            - Run gosec security checks"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker-build        - Build Docker image"
	@echo "  docker-run          - Run container interactively"
	@echo "  docker-run-daemon   - Run container as daemon"
	@echo "  docker-stop         - Stop daemon container"
	@echo "  docker-clean        - Remove Docker images"
	@echo "  docker-shell        - Open shell in container"
	@echo ""
	@echo "Other:"
	@echo "  help                - Show this help"
