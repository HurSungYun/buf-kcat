# buf-kcat

A Kafka consumer CLI tool with protobuf decoding using buf. Combines the functionality of [kafkacat/kcat](https://github.com/edenhill/kcat) with protobuf message decoding for better debugging and monitoring.

## Features

- 🚀 **Protobuf decoding** - Decodes Kafka messages using protobuf definitions
- 📦 **buf.yaml based** - Uses buf for proto compilation with full dependency support
- 🎨 **Multiple output formats** - JSON, table, pretty, raw formats
- 🔧 **Familiar kafkacat interface** - Similar command-line options

## Requirements

- Go 1.21+
- `buf` CLI - for proto compilation
- A `buf.yaml` configuration file in your project

## Installation

### Using Go
```bash
go install github.com/HurSungYun/buf-kcat@latest
```

### Build from Source
```bash
git clone https://github.com/HurSungYun/buf-kcat
cd buf-kcat
go build -o buf-kcat
```

## Usage

### Basic Usage

```bash
# Consume from topic with buf.yaml
buf-kcat -b localhost:9092 -t my-topic -p /path/to/buf.yaml -m mypackage.MyMessage

# Consume last 10 messages
buf-kcat -b localhost:9092 -t my-topic -p /path/to/buf.yaml -m mypackage.MyMessage -c 10 -o end

# Follow topic (like tail -f)
buf-kcat -b localhost:9092 -t my-topic -p /path/to/buf.yaml -m mypackage.MyMessage --follow

# List available message types
buf-kcat list -p /path/to/buf.yaml
```

### Using buf.yaml

buf-kcat requires a `buf.yaml` configuration file for proto compilation:

```bash
# Pass your buf.yaml file path
buf-kcat -b localhost:9092 -t my-topic -p /path/to/buf.yaml -m mypackage.MyMessage
```

This ensures proper dependency resolution and consistent proto compilation using buf's build system.

### Command-line Options

```
Flags:
  -b, --brokers strings       Kafka brokers (default [localhost:9092])
  -t, --topic string          Kafka topic (required)
  -m, --message-type string   Protobuf message type (required)
  -g, --group string          Consumer group (default "buf-kcat")
  -p, --proto string          Path to buf.yaml file (default "buf.yaml")
  -f, --format string         Output format: json, json-compact, table, raw, pretty (default "pretty")
  -o, --offset string         Start offset: beginning, end, stored (default "end")
  -c, --count int            Number of messages to consume (0 = unlimited)
  -k, --key string           Filter by message key
      --follow               Continue consuming messages
  -v, --verbose              Verbose output
  -h, --help                 Help for buf-kcat
```

## Output Formats

### Pretty (default)
Colored, compact output ideal for development:
```
[15:04:05] my-topic/0@12345 key=user-123 type=UserEvent
{
  "user_id": "123",
  "event_type": "LOGIN",
  "timestamp": "2024-01-15T15:04:05Z"
}
```

### JSON
Full JSON output with metadata:
```json
{
  "topic": "my-topic",
  "partition": 0,
  "offset": 12345,
  "timestamp": "2024-01-15T15:04:05Z",
  "key": "user-123",
  "message_type": "mypackage.UserEvent",
  "value": {
    "user_id": "123",
    "event_type": "LOGIN"
  }
}
```

### Table
Structured table format:
```
================================================================================
Topic:       my-topic
Partition:   0
Offset:      12345
Timestamp:   2024-01-15T15:04:05Z
Key:         user-123
Type:        mypackage.UserEvent
Value:
{
  "user_id": "123",
  "event_type": "LOGIN"
}
```

## Examples

### Debugging a specific message
```bash
# Get the last message from a topic
buf-kcat -b broker:9092 -t events -p ./buf.yaml -m mypackage.EventMessage -c 1 -o end -f json | jq .
```

### Following a topic with filtering
```bash
# Follow topic and show only messages with specific key
buf-kcat -b broker:9092 -t events -p ./buf.yaml -m mypackage.EventMessage --follow -k "user-123"
```

### Export messages for analysis
```bash
# Export last 1000 messages to file
buf-kcat -b broker:9092 -t events -p ./buf.yaml -m mypackage.EventMessage -c 1000 -f json > messages.jsonl
```

## How It Works

1. **Proto Loading**: 
   - Checks for `buf.yaml` in the specified directory or parent directories
   - If found, uses `buf build` to compile all protos with dependencies

2. **Message Decoding**:
   - Uses the specified message type to decode protobuf messages
   - Requires the full message type name (e.g., mypackage.MyMessage)

3. **Output**:
   - Decodes protobuf to JSON using protojson
   - Formats according to selected output format
   - Shows errors and raw hex for messages that fail to decode

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

If you find `buf-kcat` useful, please consider giving it a ⭐ on GitHub!
