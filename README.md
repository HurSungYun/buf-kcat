# buf-kcat

A Kafka consumer CLI tool with automatic protobuf decoding using buf. Combines the functionality of kafkacat/kcat with automatic protobuf message decoding for better debugging and monitoring.

## Features

- 🚀 **Automatic protobuf decoding** - Decodes Kafka messages using protobuf definitions
- 📦 **buf.yaml support** - Automatically uses buf for proto compilation if available
- 🔍 **Auto-detection** - Can auto-detect message types or use specified type
- 🎨 **Multiple output formats** - JSON, table, pretty, raw formats
- 🔧 **Familiar kafkacat interface** - Similar command-line options
- ⚡ **Franz-go powered** - Fast and efficient Kafka client

## Installation

```bash
go install github.com/HurSungYun/buf-kcat@latest
```

Or build from source:

```bash
git clone https://github.com/HurSungYun/buf-kcat
cd buf-kcat
go build -o buf-kcat
```

## Usage

### Basic Usage

```bash
# Consume from topic with auto-detection
buf-kcat -b localhost:9092 -t my-topic -p /path/to/protos

# Consume with specific message type
buf-kcat -b localhost:9092 -t my-topic -p /path/to/protos -m mypackage.MyMessage

# Consume last 10 messages
buf-kcat -b localhost:9092 -t my-topic -p /path/to/protos -c 10 -o end

# Follow topic (like tail -f)
buf-kcat -b localhost:9092 -t my-topic -p /path/to/protos --follow

# List available message types
buf-kcat list -p /path/to/protos
```

### With buf.yaml

If your project uses `buf.yaml`:

```bash
# Point to directory containing buf.yaml or its subdirectories
buf-kcat -b localhost:9092 -t my-topic -p /Users/lambert/workspace/idl

# buf-kcat will automatically detect and use buf.yaml for proto compilation
```

### Command-line Options

```
Flags:
  -b, --brokers strings        Kafka brokers (default [localhost:9092])
  -t, --topic string          Kafka topic (required)
  -g, --group string          Consumer group (default "buf-kcat")
  -p, --proto string          Directory containing buf.yaml or proto files (default ".")
  -m, --message-type string   Protobuf message type (auto-detect if not specified)
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
buf-kcat -b broker:9092 -t events -p ./protos -c 1 -o end -f json | jq .
```

### Following a topic with filtering
```bash
# Follow topic and show only messages with specific key
buf-kcat -b broker:9092 -t events -p ./protos --follow -k "user-123"
```

### Export messages for analysis
```bash
# Export last 1000 messages to file
buf-kcat -b broker:9092 -t events -p ./protos -c 1000 -f json > messages.jsonl
```

## How It Works

1. **Proto Loading**: 
   - Checks for `buf.yaml` in the specified directory or parent directories
   - If found, uses `buf build` to compile all protos with dependencies
   - Otherwise, falls back to `protoc` for compilation

2. **Message Decoding**:
   - If message type is specified, uses that type directly
   - Otherwise, attempts auto-detection by trying likely message types
   - Prioritizes types with keywords like "Event", "Message", "Envelope"

3. **Output**:
   - Decodes protobuf to JSON using protojson
   - Formats according to selected output format
   - Shows errors and raw hex for messages that fail to decode

## Requirements

- Go 1.21+
- One of:
  - `buf` CLI (recommended) - for projects with buf.yaml
  - `protoc` - for standalone proto files

## Comparison with kafkacat/kcat

| Feature | kafkacat/kcat | buf-kcat |
|---------|--------------|----------|
| Consume messages | ✅ | ✅ |
| Produce messages | ✅ | ❌ (planned) |
| Protobuf decoding | ❌ | ✅ |
| buf.yaml support | ❌ | ✅ |
| Auto-detect message type | ❌ | ✅ |
| JSON output | ✅ | ✅ |
| Colored output | ❌ | ✅ |

## License

MIT