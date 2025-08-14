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
	Short: "Kafka client with automatic protobuf encoding/decoding using buf",
	Long: `buf-kcat is a Kafka client that automatically encodes and decodes protobuf messages
using buf.yaml configuration. It combines the functionality of kafkacat with
automatic protobuf handling for better debugging and monitoring.

Consumer mode (default):
  buf-kcat -b localhost:9092 -t my-topic -p /path/to/buf.yaml -m mypackage.MyMessage

Producer mode:
  buf-kcat produce -b localhost:9092 -t my-topic -p /path/to/buf.yaml -m mypackage.MyMessage

List message types:
  buf-kcat list -p /path/to/buf.yaml`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Persistent flags (available to all commands)
	rootCmd.PersistentFlags().StringVarP(&protoDir, "proto", "p", "buf.yaml", "Path to buf.yaml file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	
	// Add list command
	rootCmd.AddCommand(listCmd)
}