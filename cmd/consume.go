package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/HurSungYun/buf-kcat/internal/kafka"
	"github.com/spf13/cobra"
)

var consumerCmd = &cobra.Command{
	Use:   "consume",
	Short: "Consume messages from Kafka topic (default command)",
	Long: `Consume and decode protobuf messages from a Kafka topic.

This is the default command when no subcommand is specified.

Examples:
  # Consume from topic
  buf-kcat consume -b localhost:9092 -t my-topic -p buf.yaml -m mypackage.MyMessage
  
  # Or use without 'consume' (default command)
  buf-kcat -b localhost:9092 -t my-topic -p buf.yaml -m mypackage.MyMessage`,
	Run: runConsume,
}

func init() {
	// Consumer-specific flags
	consumerCmd.Flags().StringSliceVarP(&brokers, "brokers", "b", []string{"localhost:9092"}, "Kafka brokers (comma-separated)")
	consumerCmd.Flags().StringVarP(&topic, "topic", "t", "", "Kafka topic (required)")
	consumerCmd.Flags().StringVarP(&group, "group", "g", "buf-kcat", "Consumer group")
	consumerCmd.Flags().StringVarP(&messageType, "message-type", "m", "", "Protobuf message type (required)")
	consumerCmd.Flags().StringVarP(&outputFormat, "format", "f", "json", "Output format: json, json-compact, table, raw, pretty")
	consumerCmd.Flags().StringVarP(&offset, "offset", "o", "end", "Start offset: beginning, end, stored, or timestamp:UNIX_MS")
	consumerCmd.Flags().IntVarP(&count, "count", "c", 0, "Number of messages to consume (0 = unlimited)")
	consumerCmd.Flags().BoolVar(&follow, "follow", false, "Continue consuming messages (like tail -f)")
	consumerCmd.Flags().StringVarP(&keyFilter, "key", "k", "", "Filter by message key (exact match)")
	consumerCmd.Flags().StringVarP(&protoDir, "proto", "p", "buf.yaml", "Path to buf.yaml file")
	consumerCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	_ = consumerCmd.MarkFlagRequired("topic")
	_ = consumerCmd.MarkFlagRequired("message-type")

	// Also add the same flags to root for backward compatibility
	rootCmd.Flags().StringSliceVarP(&brokers, "brokers", "b", []string{"localhost:9092"}, "Kafka brokers (comma-separated)")
	rootCmd.Flags().StringVarP(&topic, "topic", "t", "", "Kafka topic (required)")
	rootCmd.Flags().StringVarP(&group, "group", "g", "buf-kcat", "Consumer group")
	rootCmd.Flags().StringVarP(&messageType, "message-type", "m", "", "Protobuf message type (required)")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", "json", "Output format: json, json-compact, table, raw, pretty")
	rootCmd.Flags().StringVarP(&offset, "offset", "o", "end", "Start offset: beginning, end, stored, or timestamp:UNIX_MS")
	rootCmd.Flags().IntVarP(&count, "count", "c", 0, "Number of messages to consume (0 = unlimited)")
	rootCmd.Flags().BoolVar(&follow, "follow", false, "Continue consuming messages (like tail -f)")
	rootCmd.Flags().StringVarP(&keyFilter, "key", "k", "", "Filter by message key (exact match)")

	// Mark required flags for root command as well
	_ = rootCmd.MarkFlagRequired("topic")
	_ = rootCmd.MarkFlagRequired("message-type")

	// Set default command behavior
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, run consume
		runConsume(cmd, args)
	}

	rootCmd.AddCommand(consumerCmd)
}

func runConsume(cmd *cobra.Command, args []string) {
	// Validate required flags
	if topic == "" {
		fmt.Fprintf(os.Stderr, "Error: topic is required\n")
		fmt.Fprintf(os.Stderr, "Usage: buf-kcat -t <topic> -m <message-type> [options]\n")
		os.Exit(1)
	}
	if messageType == "" {
		fmt.Fprintf(os.Stderr, "Error: message-type is required\n")
		fmt.Fprintf(os.Stderr, "Usage: buf-kcat -t <topic> -m <message-type> [options]\n")
		os.Exit(1)
	}

	consumer, err := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers:      brokers,
		Group:        group,
		Topic:        topic,
		ProtoPath:    protoDir,
		MessageType:  messageType,
		OutputFormat: outputFormat,
		Offset:       offset,
		Count:        count,
		Follow:       follow,
		KeyFilter:    keyFilter,
		Verbose:      verbose,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer consumer.Close()

	if err := consumer.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
