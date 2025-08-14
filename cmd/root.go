package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	brokers      []string
	topic        string
	group        string
	protoDir     string
	messageType  string
	outputFormat string
	offset       string
	count        int
	follow       bool
	verbose      bool
	partitions   []int32
	keyFilter    string
)

var rootCmd = &cobra.Command{
	Use:   "buf-kcat",
	Short: "Kafka consumer with automatic protobuf decoding using buf",
	Long: `buf-kcat is a Kafka consumer that automatically decodes protobuf messages
using buf.yaml configuration. It combines the functionality of kafkacat with
automatic protobuf decoding for better debugging and monitoring.

Examples:
  # Consume from topic with auto-detection
  buf-kcat -b localhost:9092 -t my-topic -p /path/to/protos

  # Consume with specific message type
  buf-kcat -b localhost:9092 -t my-topic -p /path/to/protos -m mypackage.MyMessage

  # Consume last 10 messages
  buf-kcat -b localhost:9092 -t my-topic -p /path/to/protos -c 10 -o end

  # Follow topic (like tail -f)
  buf-kcat -b localhost:9092 -t my-topic -p /path/to/protos --follow`,
	Run: runConsume,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Persistent flags (available to all commands)
	rootCmd.PersistentFlags().StringVarP(&protoDir, "proto", "p", ".", "Directory containing buf.yaml or proto files")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	
	// Flags specific to consume command
	rootCmd.Flags().StringSliceVarP(&brokers, "brokers", "b", []string{"localhost:9092"}, "Kafka brokers (comma-separated)")
	rootCmd.Flags().StringVarP(&topic, "topic", "t", "", "Kafka topic (required)")
	rootCmd.Flags().StringVarP(&group, "group", "g", "buf-kcat", "Consumer group")
	rootCmd.Flags().StringVarP(&messageType, "message-type", "m", "", "Protobuf message type (auto-detect if not specified)")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", "pretty", "Output format: json, json-compact, table, raw, pretty")
	rootCmd.Flags().StringVarP(&offset, "offset", "o", "end", "Start offset: beginning, end, stored, or timestamp:UNIX_MS")
	rootCmd.Flags().IntVarP(&count, "count", "c", 0, "Number of messages to consume (0 = unlimited)")
	rootCmd.Flags().BoolVar(&follow, "follow", false, "Continue consuming messages (like tail -f)")
	rootCmd.Flags().StringVarP(&keyFilter, "key", "k", "", "Filter by message key (exact match)")

	rootCmd.MarkFlagRequired("topic")
}

func init() {
	// Add list command
	rootCmd.AddCommand(listCmd)
}