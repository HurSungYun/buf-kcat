package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	testTopic    = "test-events"
	kafkaBroker  = "localhost:9092"
	testTimeout  = 30 * time.Second
	bufKcatBin   = "../../buf-kcat"
)

func TestMain(m *testing.M) {
	// Build buf-kcat binary
	fmt.Println("Building buf-kcat binary...")
	cmd := exec.Command("go", "build", "-o", "buf-kcat", ".")
	cmd.Dir = filepath.Join("..", "..")
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Failed to build buf-kcat: %v\nOutput: %s\n", err, output)
		os.Exit(1)
	}

	// Start docker compose
	fmt.Println("Starting Kafka with docker-compose...")
	cmd = exec.Command("docker-compose", "-f", "../../docker-compose.test.yml", "up", "-d")
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to start docker-compose: %v\n", err)
		os.Exit(1)
	}

	// Wait for Kafka to be ready
	fmt.Println("Waiting for Kafka to be ready...")
	if err := waitForKafka(); err != nil {
		fmt.Printf("Kafka failed to start: %v\n", err)
		stopDocker()
		os.Exit(1)
	}

	// Create test topic
	fmt.Println("Creating test topic...")
	if err := createTopic(testTopic); err != nil {
		fmt.Printf("Failed to create topic: %v\n", err)
		stopDocker()
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	fmt.Println("Stopping docker-compose...")
	stopDocker()
	os.Exit(code)
}

func stopDocker() {
	cmd := exec.Command("docker-compose", "-f", "../../docker-compose.test.yml", "down", "-v")
	_ = cmd.Run()
}

func waitForKafka() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for Kafka")
		default:
			client, err := kgo.NewClient(kgo.SeedBrokers(kafkaBroker))
			if err == nil {
				client.Close()
				return nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func createTopic(topic string) error {
	// Create topic using kafka CLI in docker
	cmd := exec.Command("docker", "exec", "buf-kcat-test-kafka",
		"kafka-topics", "--bootstrap-server", "localhost:9092",
		"--create", "--if-not-exists",
		"--topic", topic,
		"--partitions", "3",
		"--replication-factor", "1")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if error is because topic already exists
		if strings.Contains(string(output), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to create topic: %v, output: %s", err, output)
	}
	return nil
}

func TestListCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, bufKcatBin, "list", "-p", "../example/buf.yaml")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("List command failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "events.UserEvent") {
		t.Errorf("Expected events.UserEvent in list output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "events.OrderEvent") {
		t.Errorf("Expected events.OrderEvent in list output, got: %s", outputStr)
	}
}

func TestProduceAndConsume(t *testing.T) {
	// Test data
	testMessage := map[string]interface{}{
		"user_id":    "test-user-123",
		"event_type": "LOGIN",
		"metadata": map[string]string{
			"ip": "127.0.0.1",
		},
	}

	messageJSON, err := json.Marshal(testMessage)
	if err != nil {
		t.Fatal(err)
	}

	// Produce message
	t.Run("Produce", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, bufKcatBin, "produce",
			"-b", kafkaBroker,
			"-t", testTopic,
			"-p", "../example/buf.yaml",
			"-m", "events.UserEvent",
			"-k", "test-key")

		cmd.Stdin = bytes.NewReader(messageJSON)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Produce failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(string(output), "Produced message") {
			t.Errorf("Unexpected produce output: %s", output)
		}
	})

	// Give Kafka time to process
	time.Sleep(2 * time.Second)

	// Consume message
	t.Run("Consume", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, bufKcatBin,
			"-b", kafkaBroker,
			"-t", testTopic,
			"-p", "../example/buf.yaml",
			"-m", "events.UserEvent",
			"-c", "1",
			"-o", "beginning",
			"-f", "json")

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Consume failed: %v\nOutput: %s", err, output)
		}

		// Parse the JSON output - look for the JSON object in the output
		outputStr := string(output)
		
		// Find the JSON object in the output (may have status messages before it)
		startIdx := strings.Index(outputStr, "{")
		if startIdx == -1 {
			t.Fatalf("No JSON output found: %s", outputStr)
		}
		
		jsonStr := outputStr[startIdx:]
		endIdx := strings.LastIndex(jsonStr, "}")
		if endIdx != -1 {
			jsonStr = jsonStr[:endIdx+1]
		}
		
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			t.Fatalf("Failed to parse JSON output: %v\nJSON: %s", err, jsonStr)
		}

		// Verify the message
		if result["key"] != "test-key" {
			t.Errorf("Expected key 'test-key', got %v", result["key"])
		}
		
		value, ok := result["value"].(map[string]interface{})
		if !ok {
			t.Errorf("Value is not a map: %v", result["value"])
			return
		}

		if value["user_id"] != "test-user-123" {
			t.Errorf("Expected user_id 'test-user-123', got %v", value["user_id"])
		}
		if value["event_type"] != "LOGIN" {
			t.Errorf("Expected event_type 'LOGIN', got %v", value["event_type"])
		}
	})
}

func TestProduceMultipleMessages(t *testing.T) {
	// Create a file with multiple JSON messages
	messages := []map[string]interface{}{
		{"user_id": "user-1", "event_type": "SIGNUP"},
		{"user_id": "user-2", "event_type": "LOGIN"},
		{"user_id": "user-3", "event_type": "LOGOUT"},
	}

	var buffer bytes.Buffer
	for _, msg := range messages {
		data, _ := json.Marshal(msg)
		buffer.Write(data)
		buffer.WriteByte('\n')
	}

	tmpFile, err := os.CreateTemp("", "test-messages-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(buffer.Bytes()); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Produce from file
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, bufKcatBin, "produce",
		"-b", kafkaBroker,
		"-t", testTopic,
		"-p", "../example/buf.yaml",
		"-m", "events.UserEvent",
		"-F", tmpFile.Name())

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Produce from file failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(string(output), "Produced 3 messages") {
		t.Errorf("Expected 'Produced 3 messages' in output: %s", output)
	}
}

func TestConsumeWithDifferentFormats(t *testing.T) {
	formats := []string{"json", "json-compact", "table", "pretty", "raw"}

	// First produce a test message
	testMessage := map[string]interface{}{
		"user_id":    "format-test-user",
		"event_type": "FORMAT_TEST",
	}
	messageJSON, _ := json.Marshal(testMessage)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, bufKcatBin, "produce",
		"-b", kafkaBroker,
		"-t", testTopic,
		"-p", "../example/buf.yaml",
		"-m", "events.UserEvent",
		"-k", "format-test-key")
	cmd.Stdin = bytes.NewReader(messageJSON)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to produce test message: %v\nOutput: %s", err, output)
	}

	time.Sleep(1 * time.Second)

	for _, format := range formats {
		t.Run(fmt.Sprintf("Format_%s", format), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			cmd := exec.CommandContext(ctx, bufKcatBin,
				"-b", kafkaBroker,
				"-t", testTopic,
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
				"-k", "format-test-key",
				"-c", "1",
				"-o", "beginning",
				"-f", format)

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Consume with format %s failed: %v\nOutput: %s", format, err, output)
			}

			outputStr := string(output)
			switch format {
			case "json", "json-compact":
				// Should contain valid JSON
				if !strings.Contains(outputStr, "format-test-user") {
					t.Errorf("Format %s: expected user_id in output: %s", format, outputStr)
				}
			case "table":
				// Should contain table headers
				if !strings.Contains(outputStr, "Topic:") || !strings.Contains(outputStr, "Key:") {
					t.Errorf("Format %s: expected table headers in output: %s", format, outputStr)
				}
			case "pretty":
				// Should contain formatted output
				if !strings.Contains(outputStr, "format-test-user") {
					t.Errorf("Format %s: expected formatted message in output: %s", format, outputStr)
				}
			case "raw":
				// Should contain the raw protobuf (will be binary)
				if len(output) == 0 {
					t.Errorf("Format %s: expected non-empty output", format)
				}
			}
		})
	}
}

func TestErrorHandling(t *testing.T) {
	t.Run("InvalidMessageType", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, bufKcatBin,
			"-b", kafkaBroker,
			"-t", testTopic,
			"-p", "../example/buf.yaml",
			"-m", "invalid.MessageType",
			"-c", "1")

		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("Expected error for invalid message type, got success. Output: %s", output)
		}
	})

	t.Run("InvalidBufFile", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, bufKcatBin,
			"-b", kafkaBroker,
			"-t", testTopic,
			"-p", "nonexistent/buf.yaml",
			"-m", "events.UserEvent",
			"-c", "1")

		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("Expected error for invalid buf file, got success. Output: %s", output)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, bufKcatBin, "produce",
			"-b", kafkaBroker,
			"-t", testTopic,
			"-p", "../example/buf.yaml",
			"-m", "events.UserEvent")

		cmd.Stdin = strings.NewReader("{ invalid json }")
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("Expected error for invalid JSON, got success. Output: %s", output)
		}
	})
}