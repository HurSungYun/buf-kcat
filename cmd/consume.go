package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/HurSungYun/buf-kcat/internal/decoder"
	"github.com/HurSungYun/buf-kcat/internal/formatter"
	"github.com/spf13/cobra"
	"github.com/twmb/franz-go/pkg/kgo"
)

func runConsume(cmd *cobra.Command, args []string) {
	// Initialize proto decoder
	dec, err := decoder.NewDecoder(protoDir, messageType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize decoder: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Loaded %d message types from %s\n", dec.MessageTypeCount(), protoDir)
		fmt.Fprintf(os.Stderr, "Using message type: %s\n", messageType)
	}

	// Initialize formatter
	fmtr, err := formatter.New(outputFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid output format: %v\n", err)
		os.Exit(1)
	}

	// Create Kafka client
	opts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
	}

	// Set offset
	switch {
	case strings.HasPrefix(offset, "timestamp:"):
		// Parse timestamp offset
		// For simplicity, using end for now
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()))
	case offset == "beginning":
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	case offset == "stored":
		// Use stored offset (default behavior)
	default: // "end"
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()))
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Kafka client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if verbose {
			fmt.Fprintf(os.Stderr, "\nShutting down...\n")
		}
		cancel()
	}()

	// Consume messages
	messageCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		fetches := client.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return
		}

		if err := fetches.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Fetch error: %v\n", err)
			continue
		}

		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			for _, record := range p.Records {
				// Apply key filter if specified
				if keyFilter != "" && string(record.Key) != keyFilter {
					continue
				}

				// Decode the message
				decoded, msgType, err := dec.Decode(record.Value)
				if err != nil {
					if verbose {
						fmt.Fprintf(os.Stderr, "Failed to decode message at offset %d: %v\n", record.Offset, err)
					}
					// Output raw message on decode failure
					output := formatter.Message{
						Topic:     record.Topic,
						Partition: record.Partition,
						Offset:    record.Offset,
						Key:       string(record.Key),
						Timestamp: record.Timestamp,
						Error:     err.Error(),
						RawValue:  record.Value,
					}
					fmtr.Format(output)
				} else {
					// Successfully decoded
					var value interface{}
					if err := json.Unmarshal(decoded, &value); err != nil {
						value = string(decoded)
					}

					output := formatter.Message{
						Topic:       record.Topic,
						Partition:   record.Partition,
						Offset:      record.Offset,
						Key:         string(record.Key),
						Timestamp:   record.Timestamp,
						MessageType: msgType,
						Value:       value,
					}
					fmtr.Format(output)
				}

				messageCount++
				if count > 0 && messageCount >= count {
					return
				}
			}
		})

		if count > 0 && messageCount >= count {
			break
		}

		// If not following and we've consumed existing messages, exit
		if !follow {
			// Check if we have more messages
			if fetches.NumRecords() == 0 {
				break
			}
		}
	}
}