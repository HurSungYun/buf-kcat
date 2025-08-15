.PHONY: help build test clean install release-dry release-snapshot integration-test docker-up docker-down test-all lint

# Variables
BINARY_NAME=buf-kcat
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.version=${VERSION}"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary
	go build ${LDFLAGS} -o ${BINARY_NAME} .

test: ## Run unit tests
	go test -v -race ./...

clean: ## Clean build artifacts
	go clean
	rm -f ${BINARY_NAME}
	rm -rf dist/

install: ## Install the binary
	go install ${LDFLAGS} .

deps: ## Download dependencies
	go mod download
	go mod tidy

fmt: ## Format code
	go fmt ./...
	gofmt -s -w .

release-dry: ## Test release with goreleaser (dry run)
	goreleaser release --snapshot --skip=publish --clean

release-snapshot: ## Create a snapshot release
	goreleaser release --snapshot --clean

tag: ## Create a new tag (use VERSION=v1.0.0 make tag)
	@if [ -z "${VERSION}" ]; then \
		echo "VERSION is not set. Use: VERSION=v1.0.0 make tag"; \
		exit 1; \
	fi
	git tag -a ${VERSION} -m "Release ${VERSION}"
	@echo "Tag ${VERSION} created. Run 'git push origin ${VERSION}' to trigger release"

integration-test: build ## Run integration tests
	cd test/integration && go test -v -timeout 5m

docker-up: ## Start docker compose for testing
	docker compose -f docker-compose.test.yml up -d
	@echo "Waiting for Kafka to be ready..."
	@sleep 10

docker-down: ## Stop docker compose
	docker compose -f docker-compose.test.yml down -v

test-all: test integration-test ## Run all tests (unit + integration)

lint: ## Lint the code
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run; \
	else \
		go vet ./...; \
		go fmt ./...; \
	fi