package kafka

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
	"github.com/twmb/franz-go/pkg/kgo"
)

// ConsumerConfig contains configuration for creating a Consumer.
type ConsumerConfig struct {
	Brokers      []string
	Group        string
	Topic        string
	ProtoPath    string
	MessageType  string
	OutputFormat string
	Offset       string
	Count        int
	Follow       bool
	KeyFilter    string
	Verbose      bool
}

// Consumer consumes messages, decodes them using protobuf and formats output.
type Consumer struct {
	client    *kgo.Client
	decoder   *decoder.Decoder
	formatter formatter.Formatter
	cfg       ConsumerConfig
}

// NewConsumer initializes a Consumer.
func NewConsumer(cfg ConsumerConfig) (*Consumer, error) {
	if cfg.Topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	if cfg.MessageType == "" {
		return nil, fmt.Errorf("message type is required")
	}
	dec, err := decoder.NewDecoder(cfg.ProtoPath, cfg.MessageType)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize decoder: %w", err)
	}
	fmtr, err := formatter.New(cfg.OutputFormat)
	if err != nil {
		return nil, fmt.Errorf("invalid output format: %w", err)
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(cfg.Group),
		kgo.ConsumeTopics(cfg.Topic),
		kgo.DisableAutoCommit(),
	}

	switch {
	case strings.HasPrefix(cfg.Offset, "timestamp:"):
		// For simplicity, using end for now
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()))
	case cfg.Offset == "beginning":
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	case cfg.Offset == "stored":
		// default behavior
	default: // "end"
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()))
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka client: %w", err)
	}

	return &Consumer{
		client:    client,
		decoder:   dec,
		formatter: fmtr,
		cfg:       cfg,
	}, nil
}

// Close closes the underlying Kafka client.
func (c *Consumer) Close() { c.client.Close() }

// Run starts consuming based on configuration, handles signals, and prints output.
func (c *Consumer) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	go func() {
		<-sigChan
		if c.cfg.Verbose {
			fmt.Fprintf(os.Stderr, "\nShutting down...\n")
		}
		cancel()
	}()

	fmt.Fprintf(os.Stderr, "Connected to Kafka brokers: %v\n", c.cfg.Brokers)
	fmt.Fprintf(os.Stderr, "Starting to consume from topic '%s' (group: %s, offset: %s)\n", c.cfg.Topic, c.cfg.Group, c.cfg.Offset)
	if c.cfg.MessageType != "" {
		fmt.Fprintf(os.Stderr, "Message type: %s\n", c.cfg.MessageType)
	}
	if c.cfg.KeyFilter != "" {
		fmt.Fprintf(os.Stderr, "Filtering by key: %s\n", c.cfg.KeyFilter)
	}
	if c.cfg.Count > 0 {
		fmt.Fprintf(os.Stderr, "Will consume %d messages\n", c.cfg.Count)
	}
	if c.cfg.Follow {
		fmt.Fprintf(os.Stderr, "Following topic (press Ctrl+C to stop)...\n")
	}
	fmt.Fprintf(os.Stderr, "Waiting for messages...\n\n")

	messageCount := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		fetches := c.client.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return nil
		}
		if err := fetches.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Fetch error: %v\n", err)
			continue
		}

		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			for _, record := range p.Records {
				if c.cfg.KeyFilter != "" && string(record.Key) != c.cfg.KeyFilter {
					continue
				}

				decoded, msgType, err := c.decoder.Decode(record.Value)
				if err != nil {
					if c.cfg.Verbose {
						fmt.Fprintf(os.Stderr, "Failed to decode message at offset %d: %v\n", record.Offset, err)
					}
					output := formatter.Message{
						Topic:     record.Topic,
						Partition: record.Partition,
						Offset:    record.Offset,
						Key:       string(record.Key),
						Timestamp: record.Timestamp,
						Error:     err.Error(),
						RawValue:  record.Value,
					}
					if err := c.formatter.Format(output); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to format output: %v\n", err)
					}
				} else {
					var value any
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
					if err := c.formatter.Format(output); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to format output: %v\n", err)
					}
				}

				messageCount++
				if c.cfg.Count > 0 && messageCount >= c.cfg.Count {
					return
				}
			}
		})

		if c.cfg.Count > 0 && messageCount >= c.cfg.Count {
			break
		}
		if !c.cfg.Follow {
			if fetches.NumRecords() == 0 {
				break
			}
		}
	}
	return nil
}
