package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/HurSungYun/buf-kcat/internal/kafka"
	"github.com/spf13/cobra"
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
  # Produce a single message from stdin using buf.yaml
  echo '{"user_id": "123", "event_type": "LOGIN"}' | buf-kcat produce -b localhost:9092 -t events -p buf.yaml -m events.UserEvent

  # Produce using protobuf descriptor set
  echo '{"user_id": "123", "event_type": "LOGIN"}' | buf-kcat produce -b localhost:9092 -t events -p schema.desc -m events.UserEvent

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
	produceCmd.Flags().StringVarP(&protoDir, "proto", "p", "buf.yaml", "Path to buf.yaml file or protobuf descriptor set (.desc/.pb/.protoset)")
	produceCmd.Flags().StringVarP(&messageType, "message-type", "m", "", "Protobuf message type (required)")
	produceCmd.Flags().StringVarP(&produceKey, "key", "k", "", "Message key")
	produceCmd.Flags().Int32VarP(&producePartition, "partition", "P", -1, "Specific partition to produce to (-1 for auto)")
	produceCmd.Flags().StringVarP(&produceFromFile, "file", "F", "", "Read messages from file instead of stdin")
	produceCmd.Flags().StringVarP(&produceFormat, "format", "f", "json", "Input format: json, json-compact")
	produceCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	_ = produceCmd.MarkFlagRequired("topic")
	_ = produceCmd.MarkFlagRequired("message-type")

	rootCmd.AddCommand(produceCmd)
}

func runProduce(cmd *cobra.Command, args []string) {
	// Validate required flags
	if topic == "" {
		fmt.Fprintf(os.Stderr, "Error: topic is required\n")
		fmt.Fprintf(os.Stderr, "Usage: buf-kcat produce -t <topic> -m <message-type> [options]\n")
		os.Exit(1)
	}
	if messageType == "" {
		fmt.Fprintf(os.Stderr, "Error: message-type is required\n")
		fmt.Fprintf(os.Stderr, "Usage: buf-kcat produce -t <topic> -m <message-type> [options]\n")
		os.Exit(1)
	}

	// Initialize producer
	producer, err := kafka.NewProducer(kafka.ProducerConfig{
		Brokers:     brokers,
		Topic:       topic,
		ProtoPath:   protoDir,
		MessageType: messageType,
		Key:         produceKey,
		Partition:   producePartition,
		Verbose:     verbose,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer producer.Close()

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

		// Produce via internal producer
		r, err := producer.ProduceJSON(ctx, line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			continue
		}
		messageCount++
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
