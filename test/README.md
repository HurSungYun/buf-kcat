# Integration Tests

This directory contains integration tests for buf-kcat that test the full CLI functionality with a real Kafka instance.

## Structure

- `integration/` - Integration test files
- `example/` - Example protobuf definitions used for testing
  - `buf.yaml` - Buf configuration
  - `proto/` - Proto files
- `run-integration-tests.sh` - Script to run integration tests

## Running Tests

### Prerequisites

- Go 1.21+
- Docker and Docker Compose
- `buf` CLI installed

### Running Tests Locally

#### Using Make:
```bash
# Run integration tests
make integration-test

# Run with Docker setup/teardown
make docker-up
make integration-test
make docker-down
```

#### Using the test script:
```bash
./test/run-integration-tests.sh
```

#### Manual steps:
```bash
# Build the binary
go build -o buf-kcat .

# Start Kafka
docker compose -f docker-compose.test.yml up -d

# Wait for Kafka to be ready (check with docker ps)
sleep 20

# Run tests
cd test/integration
go test -v -timeout 5m

# Clean up
cd ../..
docker compose -f docker-compose.test.yml down -v
```

## Test Coverage

The integration tests cover:

1. **List Command** - Lists available message types from proto files
2. **Produce and Consume** - Tests producing JSON messages and consuming them
3. **Multiple Messages** - Tests batch producing from a file
4. **Output Formats** - Tests all output formats (json, table, pretty, raw)
5. **Error Handling** - Tests various error scenarios

## CI/CD

The tests are automatically run in GitHub Actions for:
- Every push to main/master branches
- Every pull request

The CI workflow:
1. Sets up Go environment
2. Installs buf CLI
3. Builds the binary
4. Starts Docker Compose
5. Runs integration tests
6. Cleans up resources

## Troubleshooting

### Kafka not starting
- Check Docker is running: `docker ps`
- Check logs: `docker compose -f docker-compose.test.yml logs`
- Ensure port 9092 is not in use: `lsof -i :9092`

### Tests timing out
- Increase timeout in test files or CI workflow
- Check Kafka is healthy: `docker ps` (should show "healthy")
- Ensure topics are created successfully

### buf command not found
- Install buf: https://docs.buf.build/installation
- Ensure buf is in PATH: `which buf`