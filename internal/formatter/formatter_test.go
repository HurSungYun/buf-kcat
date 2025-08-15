package formatter

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"JSON format", "json", false},
		{"JSON Compact format", "json-compact", false},
		{"Table format", "table", false},
		{"Pretty format", "pretty", false},
		{"Raw format", "raw", false},
		{"Unknown format", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("New(%q) error = %v, wantErr %v", tt.format, err, tt.wantErr)
			}
		})
	}
}

func TestJSONFormatter(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter, _ := New("json")
	msg := Message{
		Topic:       "test-topic",
		Partition:   1,
		Offset:      100,
		Timestamp:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Key:         "test-key",
		Value:       map[string]string{"field": "value"},
		MessageType: "TestMessage",
	}

	err := formatter.Format(msg)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout
	output, _ := io.ReadAll(r)

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Verify fields
	if result["topic"] != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got %v", result["topic"])
	}
	if result["partition"] != float64(1) {
		t.Errorf("Expected partition 1, got %v", result["partition"])
	}
	if result["offset"] != float64(100) {
		t.Errorf("Expected offset 100, got %v", result["offset"])
	}
	if result["key"] != "test-key" {
		t.Errorf("Expected key 'test-key', got %v", result["key"])
	}
	if result["message_type"] != "TestMessage" {
		t.Errorf("Expected message_type 'TestMessage', got %v", result["message_type"])
	}
}

func TestTableFormatter(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter, _ := New("table")
	msg := Message{
		Topic:       "test-topic",
		Partition:   1,
		Offset:      100,
		Timestamp:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Key:         "test-key",
		Value:       map[string]string{"field": "value"},
		MessageType: "TestMessage",
	}

	err := formatter.Format(msg)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout
	output, _ := io.ReadAll(r)
	outputStr := string(output)

	// Check for expected table elements
	expectedStrings := []string{
		"Topic:",
		"test-topic",
		"Partition:",
		"1",
		"Offset:",
		"100",
		"Key:",
		"test-key",
		"Type:",
		"TestMessage",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("Expected output to contain %q, got:\n%s", expected, outputStr)
		}
	}
}

func TestPrettyFormatter(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter, _ := New("pretty")
	msg := Message{
		Topic:       "test-topic",
		Partition:   1,
		Offset:      100,
		Timestamp:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Key:         "test-key",
		Value:       map[string]string{"field": "value"},
		MessageType: "TestMessage",
	}

	err := formatter.Format(msg)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout
	output, _ := io.ReadAll(r)
	outputStr := string(output)

	// Check for pretty format elements
	if !strings.Contains(outputStr, "test-topic/1@100") {
		t.Errorf("Expected output to contain 'test-topic/1@100', got:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "key=test-key") {
		t.Errorf("Expected output to contain 'key=test-key', got:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "type=TestMessage") {
		t.Errorf("Expected output to contain 'type=TestMessage', got:\n%s", outputStr)
	}
}

func TestRawFormatter(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter, _ := New("raw")
	msg := Message{
		Topic:     "test-topic",
		Partition: 1,
		Offset:    100,
		Key:       "test-key",
		Value:     map[string]string{"field": "value"},
	}

	err := formatter.Format(msg)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout
	output, _ := io.ReadAll(r)

	// Raw format should output JSON of the value
	var result map[string]string
	outputStr := strings.TrimSpace(string(output))
	if err := json.Unmarshal([]byte(outputStr), &result); err != nil {
		t.Fatalf("Failed to parse raw output as JSON: %v\nOutput: %s", err, outputStr)
	}

	if result["field"] != "value" {
		t.Errorf("Expected field=value, got %v", result)
	}
}

func TestFormatterWithError(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter, _ := New("json")
	msg := Message{
		Topic:     "test-topic",
		Partition: 1,
		Offset:    100,
		Key:       "test-key",
		Error:     "Failed to decode",
		RawValue:  []byte{0x01, 0x02, 0x03},
	}

	err := formatter.Format(msg)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout
	output, _ := io.ReadAll(r)

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Verify error fields
	if result["error"] != "Failed to decode" {
		t.Errorf("Expected error 'Failed to decode', got %v", result["error"])
	}
	if result["raw_value_hex"] != "010203" {
		t.Errorf("Expected raw_value_hex '010203', got %v", result["raw_value_hex"])
	}
}

func TestShortTypeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"com.example.Message", "Message"},
		{"Message", "Message"},
		{"com.example.package.v1.UserEvent", "UserEvent"},
		{"", ""},
	}

	for _, tt := range tests {
		result := shortTypeName(tt.input)
		if result != tt.expected {
			t.Errorf("shortTypeName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
