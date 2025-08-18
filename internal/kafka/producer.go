package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/HurSungYun/buf-kcat/internal/decoder"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
)

// ProducerConfig contains configuration for creating a Producer.
type ProducerConfig struct {
	Brokers     []string
	Topic       string
	ProtoPath   string
	MessageType string
	Key         string
	Partition   int32
	Verbose     bool
}

// Producer wraps Kafka client and protobuf encoder for producing messages.
type Producer struct {
	client      *kgo.Client
	decoder     *decoder.Decoder
	messageType string
	topic       string
	partition   int32
	key         string
	verbose     bool
}

// NewProducer initializes a Producer.
func NewProducer(cfg ProducerConfig) (*Producer, error) {
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

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.DefaultProduceTopic(cfg.Topic),
	}
	if cfg.Partition >= 0 {
		opts = append(opts, kgo.RecordPartitioner(kgo.ManualPartitioner()))
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka client: %w", err)
	}

	return &Producer{
		client:      client,
		decoder:     dec,
		messageType: cfg.MessageType,
		topic:       cfg.Topic,
		partition:   cfg.Partition,
		key:         cfg.Key,
		verbose:     cfg.Verbose,
	}, nil
}

// Close closes the underlying Kafka client.
func (p *Producer) Close() { p.client.Close() }

// ProduceJSON validates JSON, encodes to protobuf, and produces it to Kafka.
func (p *Producer) ProduceJSON(ctx context.Context, jsonLine string) (*kgo.Record, error) {
	if jsonLine == "" {
		return nil, fmt.Errorf("empty input")
	}

	var jsonObj interface{}
	if err := json.Unmarshal([]byte(jsonLine), &jsonObj); err != nil {
		return nil, fmt.Errorf("Invalid JSON: %w", err)
	}

	protoBytes, err := encodeMessage(p.decoder, p.messageType, []byte(jsonLine))
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	record := &kgo.Record{
		Topic:     p.topic,
		Value:     protoBytes,
		Partition: p.partition,
	}
	if p.key != "" {
		record.Key = []byte(p.key)
	}

	result := p.client.ProduceSync(ctx, record)
	if err := result.FirstErr(); err != nil {
		return nil, fmt.Errorf("failed to produce message: %w", err)
	}
	r, _ := result.First()
	return r, nil
}

// encodeMessage converts JSON to protobuf bytes using the configured decoder.
func encodeMessage(dec *decoder.Decoder, msgTypeName string, jsonData []byte) ([]byte, error) {
	msgTypes := dec.GetMessageTypes()
	msgType, ok := msgTypes[msgTypeName]
	if !ok {
		return nil, fmt.Errorf("message type not found: %s", msgTypeName)
	}

	msg := dynamicpb.NewMessage(msgType.Descriptor())
	unmarshaler := protojson.UnmarshalOptions{
		DiscardUnknown: false,
		AllowPartial:   false,
	}
	if err := unmarshaler.Unmarshal(jsonData, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to proto: %w", err)
	}

	protoBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto: %w", err)
	}
	return protoBytes, nil
}
