.PHONY: help build test clean install release-dry release-snapshot

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

test: ## Run tests
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