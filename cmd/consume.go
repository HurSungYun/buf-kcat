package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/HurSungYun/buf-kcat/internal/kafka"
	"github.com/spf13/cobra"
)

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
