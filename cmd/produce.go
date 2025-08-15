package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/HurSungYun/buf-kcat/internal/decoder"
	"github.com/spf13/cobra"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
)

var (
	produceKey       string
	producePartition int32
	produceFromFile  string
	produceFormat    string
)

var produceCmd = &cobra.Command{
	Use:   "produce",
	Short: "Produce messages to Kafka topic",
	Long: `Produce protobuf messages to a Kafka topic.

Messages are read from stdin (one per line) or from a file in JSON format.
The JSON is converted to protobuf using the specified message type.

Examples:
  # Produce a single message from stdin
  echo '{"user_id": "123", "event_type": "LOGIN"}' | buf-kcat produce -b localhost:9092 -t events -p buf.yaml -m events.UserEvent

  # Produce multiple messages from file
  buf-kcat produce -b localhost:9092 -t events -p buf.yaml -m events.UserEvent -F messages.json

  # Produce with specific key
  echo '{"order_id": "456"}' | buf-kcat produce -b localhost:9092 -t orders -p buf.yaml -m orders.Order -k "order-456"

  # Interactive mode - type JSON messages, one per line
  buf-kcat produce -b localhost:9092 -t events -p buf.yaml -m events.UserEvent`,
	Run: runProduce,
}

func init() {
	produceCmd.Flags().StringSliceVarP(&brokers, "brokers", "b", []string{"localhost:9092"}, "Kafka brokers (comma-separated)")
	produceCmd.Flags().StringVarP(&topic, "topic", "t", "", "Kafka topic (required)")
	produceCmd.Flags().StringVarP(&protoDir, "proto", "p", "buf.yaml", "Path to buf.yaml file")
	produceCmd.Flags().StringVarP(&messageType, "message-type", "m", "", "Protobuf message type (required)")
	produceCmd.Flags().StringVarP(&produceKey, "key", "k", "", "Message key")
	produceCmd.Flags().Int32VarP(&producePartition, "partition", "P", -1, "Specific partition to produce to (-1 for auto)")
	produceCmd.Flags().StringVarP(&produceFromFile, "file", "F", "", "Read messages from file instead of stdin")
	produceCmd.Flags().StringVarP(&produceFormat, "format", "f", "json", "Input format: json, json-compact")
	produceCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	produceCmd.MarkFlagRequired("topic")
	produceCmd.MarkFlagRequired("message-type")
	
	rootCmd.AddCommand(produceCmd)
}

func runProduce(cmd *cobra.Command, args []string) {
	// Initialize proto decoder (also used for encoding)
	dec, err := decoder.NewDecoder(protoDir, messageType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize decoder: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Loaded %d message types from %s\n", dec.MessageTypeCount(), protoDir)
		fmt.Fprintf(os.Stderr, "Using message type: %s\n", messageType)
	}

	// Create Kafka producer client
	opts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.DefaultProduceTopic(topic),
	}

	if producePartition >= 0 {
		opts = append(opts, kgo.RecordPartitioner(kgo.ManualPartitioner()))
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Kafka client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Print connection status
	fmt.Fprintf(os.Stderr, "Connected to Kafka brokers: %v\n", brokers)
	fmt.Fprintf(os.Stderr, "Producing to topic '%s'\n", topic)
	fmt.Fprintf(os.Stderr, "Message type: %s\n", messageType)
	if produceKey != "" {
		fmt.Fprintf(os.Stderr, "Using key: %s\n", produceKey)
	}
	if producePartition >= 0 {
		fmt.Fprintf(os.Stderr, "Producing to partition: %d\n", producePartition)
	}

	// Determine input source
	var input *os.File
	if produceFromFile != "" {
		file, err := os.Open(produceFromFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		input = file
		fmt.Fprintf(os.Stderr, "Reading messages from file: %s\n", produceFromFile)
	} else {
		input = os.Stdin
		fmt.Fprintf(os.Stderr, "Reading messages from stdin (type JSON, press Enter to send, Ctrl+D to exit)...\n\n")
	}

	// Process messages
	scanner := bufio.NewScanner(input)
	messageCount := 0
	ctx := context.Background()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse JSON input
		jsonData := []byte(line)
		
		// Validate JSON
		var jsonObj interface{}
		if err := json.Unmarshal(jsonData, &jsonObj); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid JSON: %v\n", err)
			continue
		}

		// Convert JSON to protobuf
		protoBytes, err := encodeMessage(dec, messageType, jsonData)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to encode message: %v\n", err)
			continue
		}

		// Create Kafka record
		record := &kgo.Record{
			Topic:     topic,
			Value:     protoBytes,
			Partition: producePartition,
		}

		if produceKey != "" {
			record.Key = []byte(produceKey)
		}

		// Send message
		result := client.ProduceSync(ctx, record)
		if err := result.FirstErr(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to produce message: %v\n", err)
			continue
		}

		messageCount++
		r, _ := result.First()
		fmt.Fprintf(os.Stderr, "Produced message %d to %s/%d@%d\n", 
			messageCount, r.Topic, r.Partition, r.Offset)
		
		if verbose {
			fmt.Fprintf(os.Stderr, "  JSON: %s\n", line)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\nProduced %d messages successfully\n", messageCount)
}

// encodeMessage converts JSON to protobuf bytes
func encodeMessage(dec *decoder.Decoder, msgTypeName string, jsonData []byte) ([]byte, error) {
	// Get the message type
	msgTypes := dec.GetMessageTypes()
	msgType, ok := msgTypes[msgTypeName]
	if !ok {
		return nil, fmt.Errorf("message type not found: %s", msgTypeName)
	}

	// Create a new message instance
	msg := dynamicpb.NewMessage(msgType.Descriptor())

	// Unmarshal JSON into the message
	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: false,
		AllowPartial:   false,
	}
	
	if err := unmarshaler.Unmarshal(jsonData, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to proto: %w", err)
	}

	// Marshal to protobuf binary
	protoBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto: %w", err)
	}

	return protoBytes, nil
}