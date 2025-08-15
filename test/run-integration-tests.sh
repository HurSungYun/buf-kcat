#!/bin/bash
set -e

echo "Building buf-kcat binary..."
go build -o buf-kcat .

echo "Starting Kafka with docker-compose..."
docker compose -f docker-compose.test.yml up -d

echo "Waiting for Kafka to be ready..."
max_attempts=30
attempt=0
while ! docker exec buf-kcat-test-kafka kafka-topics --bootstrap-server localhost:9092 --list &>/dev/null; do
    attempt=$((attempt + 1))
    if [ $attempt -ge $max_attempts ]; then
        echo "Kafka failed to start after $max_attempts attempts"
        docker-compose -f docker-compose.test.yml down -v
        exit 1
    fi
    echo "Waiting for Kafka... attempt $attempt/$max_attempts"
    sleep 2
done

echo "Kafka is ready!"

echo "Running integration tests..."
cd test/integration
go test -v -timeout 5m

test_result=$?

echo "Cleaning up..."
cd ../..
docker compose -f docker-compose.test.yml down -v

exit $test_result