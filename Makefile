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

test: ## Run unit tests (excluding integration)
	go test -v -race $$(go list ./... | grep -v /test/integration)

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

integration-test: build ## Run integration tests (manages Docker automatically)
	cd test/integration && go test -v -timeout 5m

integration-test-only: build ## Run integration tests without Docker setup (assumes Docker is running)
	cd test/integration && SKIP_DOCKER_SETUP=true go test -v -timeout 5m

docker-up: ## Start docker compose for testing
	docker compose -f docker-compose.test.yml up -d
	@echo "Waiting for Kafka to be ready..."
	@for i in $$(seq 1 30); do \
		docker exec buf-kcat-test-kafka kafka-topics --bootstrap-server localhost:9092 --list >/dev/null 2>&1 && break || sleep 1; \
	done
	@echo "Kafka is ready!"

docker-down: ## Stop docker compose
	docker compose -f docker-compose.test.yml down -v

test-all: test integration-test ## Run all tests (unit + integration)

test-with-docker: build docker-up ## Run tests with manual Docker management
	cd test/integration && SKIP_DOCKER_SETUP=true go test -v -timeout 5m || (make docker-down && exit 1)
	make docker-down

lint: ## Lint the code
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run; \
	else \
		go vet ./...; \
		go fmt ./...; \
	fi