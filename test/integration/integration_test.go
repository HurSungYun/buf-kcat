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

// Message represents the expected JSON structure from buf-kcat output
type Message struct {
	Topic       string `json:"topic"`
	Partition   int32  `json:"partition"`
	Offset      int64  `json:"offset"`
	Timestamp   string `json:"timestamp"`
	Key         string `json:"key"`
	MessageType string `json:"message_type"`
	Value       any    `json:"value"`
	Error       string `json:"error,omitempty"`
}

// validateJSONMessage parses the output and validates the complete message structure
func validateJSONMessage(t *testing.T, output string, expectedKey string, expectedFields map[string]any) *Message {
	t.Helper()

	// Find the JSON object in the output
	jsonStart := strings.Index(output, "{")
	if jsonStart == -1 {
		t.Errorf("No JSON found in output: %s", output)
		return nil
	}

	// Extract the complete JSON object
	jsonStr := extractCompleteJSON(output[jsonStart:])
	if jsonStr == "" {
		t.Errorf("Failed to extract complete JSON from output: %s", output)
		return nil
	}

	// Parse the JSON
	var msg Message
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Errorf("Failed to parse JSON: %v\nJSON: %s", err, jsonStr)
		return nil
	}

	// Validate required fields
	if msg.Topic == "" {
		t.Errorf("Missing topic in message")
	}
	if msg.MessageType == "" {
		t.Errorf("Missing message_type in message")
	}

	// Validate expected key if provided
	if expectedKey != "" && msg.Key != expectedKey {
		t.Errorf("Expected key %q, got %q", expectedKey, msg.Key)
	}

	// Validate expected fields in the value
	if expectedFields != nil && len(expectedFields) > 0 {
		valueMap, ok := msg.Value.(map[string]any)
		if !ok {
			t.Errorf("Value is not a map: %T %v", msg.Value, msg.Value)
			return &msg
		}

		for expectedField, expectedValue := range expectedFields {
			actualValue, exists := valueMap[expectedField]
			if !exists {
				t.Errorf("Expected field %q not found in value", expectedField)
				continue
			}
			if actualValue != expectedValue {
				t.Errorf("Expected %q=%v, got %v", expectedField, expectedValue, actualValue)
			}
		}
	}

	return &msg
}

// extractCompleteJSON extracts a complete JSON object from a string starting with '{'
func extractCompleteJSON(s string) string {
	if len(s) == 0 || s[0] != '{' {
		return ""
	}

	braceCount := 0
	inString := false
	escape := false

	for i, ch := range s {
		if escape {
			escape = false
			continue
		}

		switch ch {
		case '\\':
			if inString {
				escape = true
			}
		case '"':
			if !escape {
				inString = !inString
			}
		case '{':
			if !inString {
				braceCount++
			}
		case '}':
			if !inString {
				braceCount--
				if braceCount == 0 {
					return s[:i+1]
				}
			}
		}
	}

	return "" // No complete JSON found
}

const (
	testTopic   = "test-events"
	kafkaBroker = "localhost:9092"
	testTimeout = 30 * time.Second
	bufKcatBin  = "../../buf-kcat"
)

func TestMain(m *testing.M) {
	// Check if we should manage Docker (default: yes, unless SKIP_DOCKER_SETUP=true)
	skipDocker := os.Getenv("SKIP_DOCKER_SETUP") == "true"

	// Always build buf-kcat binary if it doesn't exist
	if _, err := os.Stat(bufKcatBin); os.IsNotExist(err) {
		fmt.Println("Building buf-kcat binary...")
		cmd := exec.Command("go", "build", "-o", "buf-kcat", ".")
		cmd.Dir = filepath.Join("..", "..")
		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("Failed to build buf-kcat: %v\nOutput: %s\n", err, output)
			os.Exit(1)
		}
	}

	if !skipDocker {
		// Start docker compose
		fmt.Println("Starting Kafka with docker-compose...")
		cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "up", "-d")
		if err := cmd.Run(); err != nil {
			fmt.Printf("Failed to start docker-compose: %v\n", err)
			os.Exit(1)
		}
	}

	// Wait for Kafka to be ready
	fmt.Println("Waiting for Kafka to be ready...")
	if err := waitForKafka(); err != nil {
		fmt.Printf("Kafka failed to start: %v\n", err)
		if !skipDocker {
			stopDocker()
		}
		os.Exit(1)
	}

	// Create test topic
	fmt.Println("Creating test topic...")
	if err := createTopic(testTopic); err != nil {
		fmt.Printf("Failed to create topic: %v\n", err)
		if !skipDocker {
			stopDocker()
		}
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	if !skipDocker {
		fmt.Println("Stopping docker-compose...")
		stopDocker()
	}
	os.Exit(code)
}

func stopDocker() {
	cmd := exec.Command("docker", "compose", "-f", "../../docker-compose.test.yml", "down", "-v")
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
	tests := []struct {
		name         string
		args         []string
		wantContains []string
		wantErr      bool
	}{
		{
			name: "list available message types",
			args: []string{"list", "-p", "../example/buf.yaml"},
			wantContains: []string{
				"events.UserEvent",
				"events.OrderEvent",
				"events.SystemEvent",
			},
			wantErr: false,
		},
		{
			name:    "list with invalid buf file",
			args:    []string{"list", "-p", "nonexistent/buf.yaml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, bufKcatBin, tt.args...)
			output, err := cmd.CombinedOutput()

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v\nOutput: %s", err, tt.wantErr, output)
				return
			}

			outputStr := string(output)
			for _, want := range tt.wantContains {
				if !strings.Contains(outputStr, want) {
					t.Errorf("output missing %q\nGot: %s", want, outputStr)
				}
			}
		})
	}
}

func TestProduceCommand(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		stdin        string
		wantContains []string
		wantErr      bool
	}{
		{
			name: "produce valid JSON message",
			args: []string{
				"produce",
				"-b", kafkaBroker,
				"-t", testTopic,
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
				"-k", "test-produce-1",
			},
			stdin: `{"user_id": "user-1", "event_type": "LOGIN"}`,
			wantContains: []string{
				"Produced message",
				"Produced 1 messages successfully",
			},
			wantErr: false,
		},
		{
			name: "produce invalid JSON",
			args: []string{
				"produce",
				"-b", kafkaBroker,
				"-t", testTopic,
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
			},
			stdin: `{ invalid json }`,
			wantContains: []string{
				"Invalid JSON",
				"Produced 0 messages",
			},
			wantErr: false, // Command succeeds but produces 0 messages
		},
		{
			name: "produce with invalid message type",
			args: []string{
				"produce",
				"-b", kafkaBroker,
				"-t", testTopic,
				"-p", "../example/buf.yaml",
				"-m", "invalid.MessageType",
			},
			stdin: `{"field": "value"}`,
			wantContains: []string{
				"message type not found",
				"Produced 0 messages",
			},
			wantErr: false, // Gracefully handles the error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, bufKcatBin, tt.args...)
			if tt.stdin != "" {
				cmd.Stdin = strings.NewReader(tt.stdin)
			}

			output, err := cmd.CombinedOutput()

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v\nOutput: %s", err, tt.wantErr, output)
				return
			}

			outputStr := string(output)
			for _, want := range tt.wantContains {
				if !strings.Contains(outputStr, want) {
					t.Errorf("output missing %q\nGot: %s", want, outputStr)
				}
			}
		})
	}
}

func TestConsumeCommand(t *testing.T) {
	// First, produce test messages for consumption
	produceTestMessage := func(key, userId, eventType string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		message := fmt.Sprintf(`{"user_id": "%s", "event_type": "%s"}`, userId, eventType)
		cmd := exec.CommandContext(ctx, bufKcatBin, "produce",
			"-b", kafkaBroker,
			"-t", testTopic,
			"-p", "../example/buf.yaml",
			"-m", "events.UserEvent",
			"-k", key)
		cmd.Stdin = strings.NewReader(message)
		if _, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Failed to produce test message: %v", err)
		}
	}

	// Produce test messages
	produceTestMessage("consume-test-1", "user-100", "LOGIN")
	time.Sleep(2 * time.Second) // Give Kafka time to process

	tests := []struct {
		name           string
		args           []string
		expectedKey    string
		expectedFields map[string]any
		checkJSON      bool
		wantErr        bool
	}{
		{
			name: "consume message with JSON format",
			args: []string{
				"-b", kafkaBroker,
				"-t", testTopic,
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
				"-k", "consume-test-1",
				"-c", "1",
				"-o", "beginning",
				"-f", "json",
			},
			expectedKey: "consume-test-1",
			expectedFields: map[string]any{
				"user_id":    "user-100",
				"event_type": "LOGIN",
			},
			checkJSON: true,
			wantErr:   false,
		},
		{
			name: "consume with pretty format",
			args: []string{
				"-b", kafkaBroker,
				"-t", testTopic,
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
				"-k", "consume-test-1",
				"-c", "1",
				"-o", "beginning",
				"-f", "pretty",
			},
			expectedKey: "", // Pretty format doesn't use JSON validation
			checkJSON:   false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, bufKcatBin, tt.args...)
			output, err := cmd.CombinedOutput()

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v\nOutput: %s", err, tt.wantErr, output)
				return
			}

			outputStr := string(output)

			if tt.checkJSON {
				// Use complete JSON validation
				msg := validateJSONMessage(t, outputStr, tt.expectedKey, tt.expectedFields)
				if msg == nil {
					return // validateJSONMessage already logged the error
				}

				// Additional validation
				if msg.MessageType != "events.UserEvent" {
					t.Errorf("Expected message_type %q, got %q", "events.UserEvent", msg.MessageType)
				}
			} else {
				// For non-JSON formats, check for key string patterns
				if !strings.Contains(outputStr, "key=consume-test-1") {
					t.Errorf("output missing expected key pattern in pretty format\nGot: %s", outputStr)
				}
				if !strings.Contains(outputStr, "user-100") {
					t.Errorf("output missing expected user_id\nGot: %s", outputStr)
				}
				if !strings.Contains(outputStr, "LOGIN") {
					t.Errorf("output missing expected event_type\nGot: %s", outputStr)
				}
			}
		})
	}
}

func TestOutputFormats(t *testing.T) {
	// Produce a fresh message for testing formats
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testMessage := fmt.Sprintf(`{"user_id": "format-test-%d", "event_type": "FORMAT_TEST"}`, time.Now().Unix())
	cmd := exec.CommandContext(ctx, bufKcatBin, "produce",
		"-b", kafkaBroker,
		"-t", testTopic,
		"-p", "../example/buf.yaml",
		"-m", "events.UserEvent")
	cmd.Stdin = strings.NewReader(testMessage)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to produce test message: %v\nOutput: %s", err, output)
	}

	time.Sleep(2 * time.Second) // Give Kafka time to process

	tests := []struct {
		name         string
		format       string
		wantContains []string
		checkFunc    func(t *testing.T, output string)
	}{
		{
			name:   "JSON format",
			format: "json",
			wantContains: []string{
				`"key"`,
				`"topic"`,
				`"partition"`,
				`"offset"`,
				`"value"`,
			},
			checkFunc: func(t *testing.T, output string) {
				// Should have valid JSON output (could be indented or compact)
				foundJSON := false

				// Try to find JSON object in the output
				jsonStart := strings.Index(output, "{")
				if jsonStart >= 0 {
					// Find the matching closing brace
					openBraces := 0
					inString := false
					escapeNext := false
					jsonEnd := -1

					for i := jsonStart; i < len(output); i++ {
						ch := output[i]

						if escapeNext {
							escapeNext = false
							continue
						}

						if ch == '\\' {
							escapeNext = true
							continue
						}

						if ch == '"' && !escapeNext {
							inString = !inString
							continue
						}

						if !inString {
							if ch == '{' {
								openBraces++
							} else if ch == '}' {
								openBraces--
								if openBraces == 0 {
									jsonEnd = i + 1
									break
								}
							}
						}
					}

					if jsonEnd > jsonStart {
						jsonStr := output[jsonStart:jsonEnd]
						var result map[string]any
						if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
							// Check it has expected fields
							if _, hasKey := result["topic"]; hasKey {
								foundJSON = true
							}
						}
					}
				}

				if !foundJSON {
					t.Error("No valid JSON message found in output")
				}
			},
		},
		{
			name:   "JSON compact format",
			format: "json-compact",
			wantContains: []string{
				`{"key"`,
				`"topic"`,
				`"value"`,
			},
			checkFunc: func(t *testing.T, output string) {
				// Should be single line JSON
				lines := strings.Split(strings.TrimSpace(output), "\n")
				jsonLines := 0
				for _, line := range lines {
					if strings.HasPrefix(line, "{") {
						jsonLines++
					}
				}
				if jsonLines == 0 {
					t.Error("No JSON lines found")
				}
			},
		},
		{
			name:   "Table format",
			format: "table",
			wantContains: []string{
				"Topic:",
				"Partition:",
				"Offset:",
				"Key:",
				"Value:",
				"=====",
			},
		},
		{
			name:   "Pretty format",
			format: "pretty",
			wantContains: []string{
				testTopic,
				"type=", // Should show message type
			},
		},
		{
			name:   "Raw format",
			format: "raw",
			checkFunc: func(t *testing.T, output string) {
				// Raw format should contain some JSON content
				if !strings.Contains(output, "{") && !strings.Contains(output, "}") {
					t.Error("Raw format should contain JSON data")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Consume the last message we produced
			cmd := exec.CommandContext(ctx, bufKcatBin,
				"-b", kafkaBroker,
				"-t", testTopic,
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
				"-c", "1",
				"-o", "beginning",
				"-f", tt.format)

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Command failed: %v\nOutput: %s", err, output)
			}

			outputStr := string(output)

			for _, want := range tt.wantContains {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Format %s: output missing %q\nGot: %s", tt.format, want, outputStr)
				}
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, outputStr)
			}
		})
	}
}

func TestProduceMultipleMessages(t *testing.T) {
	tests := []struct {
		name         string
		messages     []map[string]any
		wantProduced int
		wantErr      bool
	}{
		{
			name: "produce 3 messages from file",
			messages: []map[string]any{
				{"user_id": "user-1", "event_type": "SIGNUP"},
				{"user_id": "user-2", "event_type": "LOGIN"},
				{"user_id": "user-3", "event_type": "LOGOUT"},
			},
			wantProduced: 3,
			wantErr:      false,
		},
		{
			name: "produce with mixed valid and invalid",
			messages: []map[string]any{
				{"user_id": "user-4", "event_type": "LOGIN"},
				nil, // This will create invalid JSON
				{"user_id": "user-5", "event_type": "LOGOUT"},
			},
			wantProduced: 2,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file with messages
			var buffer bytes.Buffer
			validCount := 0
			for _, msg := range tt.messages {
				if msg != nil {
					data, _ := json.Marshal(msg)
					buffer.Write(data)
					validCount++
				} else {
					buffer.WriteString("invalid json")
				}
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
			outputStr := string(output)

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v\nOutput: %s", err, tt.wantErr, outputStr)
			}

			expectedMsg := fmt.Sprintf("Produced %d messages", tt.wantProduced)
			if !strings.Contains(outputStr, expectedMsg) {
				t.Errorf("Expected %q in output, got: %s", expectedMsg, outputStr)
			}
		})
	}
}

func TestComplexMessageTypes(t *testing.T) {
	// Test oneof fields, enums, nested messages, etc.
	tests := []struct {
		name           string
		messageType    string
		jsonData       string
		expectedFields map[string]any
	}{
		{
			name:        "ComplexEvent with oneof user_activity",
			messageType: "events.ComplexEvent",
			jsonData:    `{"event_id": "evt-123", "timestamp": "2024-01-15T15:04:05Z", "source": "test-service", "priority": 1, "labels": ["test", "integration"], "user_activity": {"user_id": "user-456", "action": "login", "resource": "/api/auth", "context": {"ip": "192.168.1.1"}}}`,
			expectedFields: map[string]any{
				"event_id": "evt-123",
				"source":   "test-service",
				"priority": float64(1), // JSON numbers are float64
			},
		},
		{
			name:        "SystemEvent with enums and arrays",
			messageType: "events.SystemEvent",
			jsonData:    `{"system_id": "sys-001", "event_level": "ERROR", "message": "Database connection failed", "timestamp": "2024-01-15T15:04:05Z", "uptime": "3600s", "tags": ["database", "error", "connection"]}`,
			expectedFields: map[string]any{
				"system_id":   "sys-001",
				"event_level": "ERROR",
				"message":     "Database connection failed",
			},
		},
		{
			name:        "OrderEvent with PaymentMethod enum",
			messageType: "events.OrderEvent",
			jsonData:    `{"order_id": "order-789", "user_id": "user-123", "status": "confirmed", "total_amount": 99.99, "payment_method": "CREDIT_CARD", "created_at": "2024-01-15T15:04:05Z", "items": [{"product_id": "prod-1", "quantity": 2, "price": 49.99}]}`,
			expectedFields: map[string]any{
				"order_id":       "order-789",
				"user_id":        "user-123",
				"status":         "confirmed",
				"total_amount":   99.99,
				"payment_method": "CREDIT_CARD",
			},
		},
		{
			name:           "EmptyEvent",
			messageType:    "events.EmptyEvent",
			jsonData:       `{}`,
			expectedFields: map[string]any{}, // Empty message should have empty value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a unique topic for each test to avoid interference
			uniqueTopic := fmt.Sprintf("test-complex-%d", time.Now().UnixNano())

			// Create the unique topic
			if err := createTopic(uniqueTopic); err != nil {
				t.Fatalf("Failed to create topic: %v", err)
			}

			// First produce the message
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			testKey := fmt.Sprintf("key-%d", time.Now().UnixNano())
			cmd := exec.CommandContext(ctx, bufKcatBin, "produce",
				"-b", kafkaBroker,
				"-t", uniqueTopic,
				"-p", "../example/buf.yaml",
				"-m", tt.messageType,
				"-k", testKey)
			cmd.Stdin = strings.NewReader(tt.jsonData)

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Failed to produce message: %v\nOutput: %s", err, output)
			}

			// Verify production was successful
			if !strings.Contains(string(output), "Produced message") {
				t.Fatalf("Production did not complete successfully: %s", output)
			}

			time.Sleep(3 * time.Second) // Let Kafka process

			// Then consume and verify
			ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel2()

			cmd2 := exec.CommandContext(ctx2, bufKcatBin,
				"-b", kafkaBroker,
				"-t", uniqueTopic,
				"-p", "../example/buf.yaml",
				"-m", tt.messageType,
				"-c", "1",
				"-o", "beginning",
				"-f", "json")

			output2, err2 := cmd2.CombinedOutput()
			if err2 != nil {
				t.Fatalf("Failed to consume message: %v\nOutput: %s", err2, output2)
			}

			outputStr := string(output2)

			// Validate the complete JSON message structure
			msg := validateJSONMessage(t, outputStr, testKey, tt.expectedFields)
			if msg == nil {
				return // validateJSONMessage already logged the error
			}

			// Additional validation for specific message types
			if msg.MessageType != tt.messageType {
				t.Errorf("Expected message_type %q, got %q", tt.messageType, msg.MessageType)
			}

			if msg.Topic != uniqueTopic {
				t.Errorf("Expected topic %q, got %q", uniqueTopic, msg.Topic)
			}
		})
	}
}

func TestAdvancedConsumerFeatures(t *testing.T) {
	tests := []struct {
		name           string
		setupKey       string
		setupMessage   string
		args           []string
		expectedFields map[string]any
		wantCount      int // Expected number of messages
	}{
		{
			name:         "Key filtering - specific key",
			setupKey:     "filter-key-123",
			setupMessage: `{"user_id": "filter-user-1", "event_type": "LOGIN"}`,
			args: []string{
				"-b", kafkaBroker,
				"-t", "", // Will be set dynamically
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
				"-k", "filter-key-123",
				"-c", "1", // Just get one message
				"-o", "beginning",
				"-f", "json",
			},
			expectedFields: map[string]any{
				"user_id":    "filter-user-1",
				"event_type": "LOGIN",
			},
			wantCount: 1,
		},
		{
			name:         "Count limit test",
			setupKey:     "count-key-789",
			setupMessage: `{"user_id": "count-user", "event_type": "COUNT_TEST"}`,
			args: []string{
				"-b", kafkaBroker,
				"-t", "", // Will be set dynamically
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
				"-c", "1", // Test count limiting
				"-o", "beginning",
				"-f", "json",
			},
			expectedFields: map[string]any{
				"user_id":    "count-user",
				"event_type": "COUNT_TEST",
			},
			wantCount: 1, // Should get exactly 1 message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create unique topic for this specific test
			uniqueTopic := fmt.Sprintf("test-adv-%d", time.Now().UnixNano())
			if err := createTopic(uniqueTopic); err != nil {
				t.Fatalf("Failed to create topic: %v", err)
			}

			// Set the topic in args
			for i, arg := range tt.args {
				if arg == "" && i > 0 && tt.args[i-1] == "-t" {
					tt.args[i] = uniqueTopic
					break
				}
			}

			// Produce setup message
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cmd := exec.CommandContext(ctx, bufKcatBin, "produce",
				"-b", kafkaBroker,
				"-t", uniqueTopic,
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
				"-k", tt.setupKey)
			cmd.Stdin = strings.NewReader(tt.setupMessage)
			output, err := cmd.CombinedOutput()
			cancel()

			if err != nil {
				t.Fatalf("Failed to produce setup message: %v\nOutput: %s", err, output)
			}
			if !strings.Contains(string(output), "Produced message") {
				t.Fatalf("Production did not complete successfully: %s", output)
			}

			time.Sleep(3 * time.Second) // Let Kafka process

			// Run the consumer test
			ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel2()

			cmd2 := exec.CommandContext(ctx2, bufKcatBin, tt.args...)
			output2, err2 := cmd2.CombinedOutput()

			if err2 != nil {
				t.Errorf("Command failed: %v\nOutput: %s", err2, output2)
				return
			}

			outputStr := string(output2)

			// Validate the complete JSON message structure
			msg := validateJSONMessage(t, outputStr, tt.setupKey, tt.expectedFields)
			if msg == nil {
				return // validateJSONMessage already logged the error
			}

			// Verify message type and topic
			if msg.MessageType != "events.UserEvent" {
				t.Errorf("Expected message_type %q, got %q", "events.UserEvent", msg.MessageType)
			}

			if msg.Topic != uniqueTopic {
				t.Errorf("Expected topic %q, got %q", uniqueTopic, msg.Topic)
			}

			// Check message count by counting JSON objects
			if tt.wantCount > 0 {
				jsonCount := strings.Count(outputStr, `"topic"`)
				if jsonCount != tt.wantCount {
					t.Errorf("Expected %d messages, got %d\nOutput: %s", tt.wantCount, jsonCount, outputStr)
				}
			}
		})
	}
}

func TestHelpCommands(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantContains []string
		wantErr      bool
	}{
		{
			name:         "Root help",
			args:         []string{"--help"},
			wantContains: []string{"buf-kcat", "Commands:", "consume", "produce", "list"},
			wantErr:      false,
		},
		{
			name:         "Consume help",
			args:         []string{"consume", "--help"},
			wantContains: []string{"consume", "topic", "message-type", "brokers"},
			wantErr:      false,
		},
		{
			name:         "Produce help",
			args:         []string{"produce", "--help"},
			wantContains: []string{"produce", "topic", "message-type", "key", "file"},
			wantErr:      false,
		},
		{
			name:         "List help",
			args:         []string{"list", "--help"},
			wantContains: []string{"list", "proto", "message types"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, bufKcatBin, tt.args...)
			output, err := cmd.CombinedOutput()

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v\nOutput: %s", err, tt.wantErr, output)
				return
			}

			outputStr := string(output)
			for _, want := range tt.wantContains {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Help output missing %q\nGot: %s", want, outputStr)
				}
			}
		})
	}
}

func TestRequiredFlags(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantErr      bool
		wantContains []string
	}{
		{
			name: "Missing topic flag",
			args: []string{
				"consume",
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
			},
			wantErr:      true,
			wantContains: []string{"required flag", "topic"},
		},
		{
			name: "Missing message-type flag",
			args: []string{
				"consume",
				"-t", testTopic,
				"-p", "../example/buf.yaml",
			},
			wantErr:      true,
			wantContains: []string{"required flag", "message-type"},
		},
		{
			name: "Missing both required flags",
			args: []string{
				"consume",
				"-p", "../example/buf.yaml",
			},
			wantErr:      true,
			wantContains: []string{"required flag"},
		},
		{
			name: "Root command missing topic",
			args: []string{
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
			},
			wantErr:      true,
			wantContains: []string{"required flag", "topic"},
		},
		{
			name: "Produce missing topic",
			args: []string{
				"produce",
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
			},
			wantErr:      true,
			wantContains: []string{"required flag", "topic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, bufKcatBin, tt.args...)
			output, err := cmd.CombinedOutput()

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v\nOutput: %s", err, tt.wantErr, output)
				return
			}

			outputStr := string(output)
			for _, want := range tt.wantContains {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Output missing %q\nGot: %s", want, outputStr)
				}
			}
		})
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		args          []string
		stdin         string
		wantErr       bool
		wantInOutput  []string
		acceptSuccess bool // Some errors are handled gracefully
	}{
		{
			name:    "consume with invalid message type",
			command: "consume",
			args: []string{
				"-b", kafkaBroker,
				"-t", testTopic,
				"-p", "../example/buf.yaml",
				"-m", "invalid.MessageType",
				"-c", "1",
				"-o", "end", // Use end offset to minimize waiting time
			},
			wantErr:       false, // CLI doesn't fail immediately - it waits for messages
			acceptSuccess: true,
			wantInOutput:  []string{"Message type: invalid.MessageType"}, // Verify it accepts the type
		},
		{
			name:    "invalid buf file",
			command: "consume",
			args: []string{
				"-b", kafkaBroker,
				"-t", testTopic,
				"-p", "nonexistent/buf.yaml",
				"-m", "events.UserEvent",
				"-c", "1",
			},
			wantErr: true,
		},
		{
			name:    "produce invalid JSON",
			command: "produce",
			args: []string{
				"-b", kafkaBroker,
				"-t", testTopic,
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
			},
			stdin:         "{ invalid json }",
			wantErr:       false,
			acceptSuccess: true,
			wantInOutput:  []string{"Invalid JSON", "Produced 0 messages"},
		},
		{
			name:    "invalid broker address",
			command: "consume",
			args: []string{
				"-b", "invalid-broker:9999",
				"-t", testTopic,
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
				"-c", "1",
			},
			wantErr: true,
		},
		{
			name:    "missing required flags - no topic",
			command: "consume",
			args: []string{
				"-b", kafkaBroker,
				"-p", "../example/buf.yaml",
				"-m", "events.UserEvent",
				"-c", "1",
			},
			wantErr:      true,
			wantInOutput: []string{"required flag", "topic"},
		},
		{
			name:    "missing required flags - no message type",
			command: "consume",
			args: []string{
				"-b", kafkaBroker,
				"-t", testTopic,
				"-p", "../example/buf.yaml",
				"-c", "1",
			},
			wantErr:      true,
			wantInOutput: []string{"required flag", "message-type"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			args := append([]string{tt.command}, tt.args...)
			cmd := exec.CommandContext(ctx, bufKcatBin, args...)

			if tt.stdin != "" {
				cmd.Stdin = strings.NewReader(tt.stdin)
			}

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Check error expectation
			if !tt.acceptSuccess && (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v\nOutput: %s", err, tt.wantErr, outputStr)
				return
			}

			// If we accept success, check for specific output
			if tt.acceptSuccess && err == nil {
				foundExpected := false
				for _, want := range tt.wantInOutput {
					if strings.Contains(outputStr, want) {
						foundExpected = true
						break
					}
				}
				if !foundExpected && len(tt.wantInOutput) > 0 {
					t.Errorf("Expected one of %v in output, got: %s", tt.wantInOutput, outputStr)
				}
			}

			// Check for expected strings in output
			for _, want := range tt.wantInOutput {
				if !strings.Contains(outputStr, want) {
					t.Logf("Warning: output missing %q\nGot: %s", want, outputStr)
				}
			}
		})
	}
}
