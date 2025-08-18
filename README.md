# buf-kcat

A Kafka client CLI tool with protobuf encoding/decoding using buf. Combines the functionality of [kafkacat/kcat](https://github.com/edenhill/kcat) with protobuf message encoding and decoding for better debugging and monitoring.

> ⚠️ **Note**: This is a debugging/development tool that requires the `buf` CLI to be installed. Currently, it uses `buf build` command internally to compile protobuf definitions, which involves executing external commands. Not recommended for production use cases where security and reliability are critical.

## Features

- 🚀 **Protobuf encoding/decoding** - Encodes and decodes Kafka messages using protobuf definitions
- 🔗 **Pipe-friendly JSON output** - Default JSON format for easy integration with jq, grep, and other tools
- ✍️ **Producer mode** - Send JSON messages that are automatically encoded to protobuf
- 📦 **buf.yaml based** - Uses buf for proto compilation with full dependency support
- 🎨 **Multiple output formats** - JSON (default), json-compact, table, pretty, raw formats
- 🔧 **Familiar kafkacat interface** - Similar command-line options

## Requirements

- Go 1.21+
- `buf` CLI - **Required**: This tool executes `buf build` command internally for proto compilation. [Install buf](https://docs.buf.build/installation)

## Installation

### Using Homebrew (macOS/Linux)
```bash
brew tap HurSungYun/tap
brew install buf-kcat
```

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

### Download Binary
Download pre-built binaries from the [releases page](https://github.com/HurSungYun/buf-kcat/releases).

## Usage

### Basic Usage

buf-kcat requires protobuf definitions (either a `buf.yaml` configuration file or a protobuf descriptor set), a topic name (`-t`), and a message type (`-m`). By default, output is in JSON format for easy integration with pipes and other tools:

```bash
# Consume from topic using buf.yaml (outputs JSON by default)
# Both -t (topic) and -m (message-type) are required
buf-kcat -b localhost:9092 -t my-topic -p /path/to/buf.yaml -m mypackage.MyMessage

# Consume using protobuf descriptor set (.desc/.pb/.protoset files)
buf-kcat -b localhost:9092 -t my-topic -p /path/to/schema.desc -m mypackage.MyMessage

# Pipe JSON output to jq for processing
buf-kcat -b localhost:9092 -t my-topic -p buf.yaml -m mypackage.MyMessage -c 10 | jq '.value'

# Use pretty format for human-readable output
buf-kcat -b localhost:9092 -t my-topic -p buf.yaml -m mypackage.MyMessage -f pretty

# Follow topic and filter with jq
buf-kcat -b localhost:9092 -t my-topic -p buf.yaml -m mypackage.MyMessage --follow | jq 'select(.value.status == "ERROR")'

# List available message types from buf.yaml
buf-kcat list -p /path/to/buf.yaml

# List available message types from descriptor set
buf-kcat list -p /path/to/schema.desc
```

### Protobuf Input Options

buf-kcat supports two ways to provide protobuf definitions:

#### 1. buf.yaml Configuration (Recommended)
```bash
# Use buf.yaml for automatic dependency resolution and compilation
buf-kcat -t my-topic -p buf.yaml -m mypackage.MyMessage
```

#### 2. Buf Image / Protobuf Descriptor Set Files (No External Commands)
```bash
# Generate buf image from buf.yaml
buf build -o schema.desc

# Or generate descriptor set from protoc
protoc --descriptor_set_out=schema.desc --include_imports *.proto

# Use buf image directly (no buf dependency required, no external commands executed)
buf-kcat -t my-topic -p schema.desc -m mypackage.MyMessage
```

**Supported formats:** Buf images (`.desc`), protobuf descriptor sets (`.pb`, `.protoset`), or any binary file containing compiled protobuf definitions.

**🔒 Security Benefit:** When using buf images/descriptor sets, buf-kcat does **not execute any external commands** - it loads protobuf definitions directly from the pre-compiled binary file.

### Producer Mode

Produce JSON messages that are automatically encoded to protobuf:

```bash
# Produce a single message from stdin using buf.yaml
echo '{"user_id": "123", "event_type": "LOGIN"}' | \
  buf-kcat produce -b localhost:9092 -t events -p buf.yaml -m events.UserEvent

# Produce using descriptor set
echo '{"user_id": "123", "event_type": "LOGIN"}' | \
  buf-kcat produce -b localhost:9092 -t events -p schema.desc -m events.UserEvent

# Produce multiple messages from a file
buf-kcat produce -b localhost:9092 -t events -p buf.yaml -m events.UserEvent -F messages.json

# Produce with a specific key
echo '{"order_id": "456", "total": 99.99}' | \
  buf-kcat produce -b localhost:9092 -t orders -p buf.yaml -m orders.Order -k "order-456"

# Interactive mode - type JSON messages, press Enter to send each
buf-kcat produce -b localhost:9092 -t events -p buf.yaml -m events.UserEvent
{"user_id": "789", "event_type": "SIGNUP"}
{"user_id": "790", "event_type": "LOGIN"}
^D  # Press Ctrl+D to exit

# Produce to specific partition
echo '{"metric": "cpu", "value": 85.5}' | \
  buf-kcat produce -b localhost:9092 -t metrics -p buf.yaml -m metrics.Metric -P 2
```

#### Producer Input Format

The producer accepts JSON input that matches your protobuf message structure:

```jsonc
// For a protobuf message:
// message UserEvent {
//   string user_id = 1;
//   string event_type = 2;
//   google.protobuf.Timestamp timestamp = 3;
// }

// Provide JSON:
{
  "user_id": "123",
  "event_type": "LOGIN",
  "timestamp": "2024-01-15T15:04:05Z"
}
```

#### Producer Output Example

```bash
$ echo '{"user_id": "123", "event_type": "LOGIN"}' | buf-kcat produce -t events -p buf.yaml -m events.UserEvent

Connected to Kafka brokers: [localhost:9092]
Producing to topic 'events'
Message type: events.UserEvent
Reading messages from stdin (type JSON, press Enter to send, Ctrl+D to exit)...

Produced message 1 to events/0@12345

Produced 1 messages successfully
```

### Pipe Integration Examples

buf-kcat outputs JSON by default, making it perfect for use with tools like `jq`, `grep`, and other Unix utilities:

```bash
# Extract specific fields
buf-kcat -t events -p buf.yaml -m events.UserEvent -c 100 | jq -r '.value.user_id'

# Filter messages by content
buf-kcat -t events -p buf.yaml -m events.UserEvent --follow | jq 'select(.value.event_type == "ERROR")'

# Count messages by type
buf-kcat -t events -p buf.yaml -m events.UserEvent -c 1000 | jq -r '.value.event_type' | sort | uniq -c

# Save to file for analysis
buf-kcat -t events -p buf.yaml -m events.UserEvent -o beginning > events.jsonl

# Monitor for specific conditions
buf-kcat -t metrics -p buf.yaml -m metrics.SystemMetric --follow | \
  jq 'select(.value.cpu_usage > 80) | {time: .timestamp, cpu: .value.cpu_usage}'

# Format as CSV
buf-kcat -t users -p buf.yaml -m user.Profile -c 100 | \
  jq -r '[.key, .value.user_id, .value.email] | @csv'
```

### Output Formats

```bash
# JSON (default) - Best for piping and processing
buf-kcat -t events -p buf.yaml -m events.UserEvent

# Pretty - Human-readable with colors
buf-kcat -t events -p buf.yaml -m events.UserEvent -f pretty

# Table - Structured view
buf-kcat -t events -p buf.yaml -m events.UserEvent -f table

# Raw - Just the decoded message value
buf-kcat -t events -p buf.yaml -m events.UserEvent -f raw

# JSON Compact - Single line JSON
buf-kcat -t events -p buf.yaml -m events.UserEvent -f json-compact
```

### Example JSON Output

When using the default JSON format, each message is output as a JSON object:

```json
{
  "topic": "events",
  "partition": 0,
  "offset": 12345,
  "timestamp": "2024-01-15T15:04:05Z",
  "key": "user-123",
  "message_type": "events.UserEvent",
  "value": {
    "user_id": "123",
    "event_type": "LOGIN",
    "timestamp": "2024-01-15T15:04:05Z",
    "metadata": {
      "ip_address": "192.168.1.1",
      "user_agent": "Mozilla/5.0"
    }
  }
}
```

#### List Command Output

The list command shows all available message types:

```bash
$ buf-kcat list -p ./protos/buf.yaml

Loaded 15 message types from ./protos/buf.yaml:

events.UserEvent
events.SystemEvent
events.OrderEvent
user.Profile
user.Settings
orders.Order
orders.OrderItem
orders.ShippingInfo
common.Timestamp
common.Money
common.Address
metrics.Counter
metrics.Gauge
metrics.Histogram
metrics.Summary
```


### Command-line Options

```
Flags:
  -b, --brokers strings       Kafka brokers (default [localhost:9092])
  -t, --topic string          Kafka topic name (REQUIRED)
  -m, --message-type string   Protobuf message type (REQUIRED)
  -g, --group string          Consumer group (default "buf-kcat")
  -p, --proto string          Path to buf.yaml file or protobuf descriptor set (.desc/.pb/.protoset) (default "buf.yaml")
  -f, --format string         Output format: json, json-compact, table, raw, pretty (default "json")
  -o, --offset string         Start offset: beginning, end, stored (default "end")
  -c, --count int            Number of messages to consume (0 = unlimited)
  -k, --key string           Filter by message key
      --follow               Continue consuming messages
  -v, --verbose              Verbose output
  -h, --help                 Help for buf-kcat
```

## Output Formats

### Pretty (default)
Colored, compact output ideal for development. The header shows timestamp, topic/partition@offset, key, and message type:
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

Example JSON output (formatted with jq):
```json
{
  "topic": "events",
  "partition": 0,
  "offset": 12345,
  "timestamp": "2024-01-15T15:04:05Z",
  "key": "user-123",
  "message_type": "mypackage.EventMessage",
  "value": {
    "event_id": "evt_abc123",
    "user_id": "123",
    "action": "purchase_completed",
    "amount": 99.99
  }
}
```

### Following a topic with filtering
```bash
# Follow topic and show only messages with specific key
buf-kcat -b broker:9092 -t events -p ./buf.yaml -m mypackage.EventMessage --follow -k "user-123"
```

This will show connection info and then only messages matching the key filter:
```
Connected to Kafka brokers: [broker:9092]
Starting to consume from topic 'events' (group: buf-kcat, offset: end)
Message type: mypackage.EventMessage
Filtering by key: user-123
Following topic (press Ctrl+C to stop)...
Waiting for messages...

[15:05:01] events/0@12350 key=user-123 type=mypackage.EventMessage
{
  "event_id": "evt_xyz789",
  "user_id": "123",
  "action": "profile_updated"
}
```

### Export messages for analysis
```bash
# Export last 1000 messages to file
buf-kcat -b broker:9092 -t events -p ./buf.yaml -m mypackage.EventMessage -c 1000 -f json-compact > messages.jsonl
```

The status messages go to stderr, so stdout contains only clean JSON:
```bash
# Terminal shows status (stderr):
Connected to Kafka brokers: [broker:9092]
Starting to consume from topic 'events' (group: buf-kcat, offset: end)
Message type: mypackage.EventMessage
Will consume 1000 messages
Waiting for messages...

# messages.jsonl contains clean JSON Lines (stdout):
{"topic":"events","partition":0,"offset":12345,"key":"user-1","timestamp":"2024-01-15T15:00:00Z","message_type":"mypackage.EventMessage","value":{"event_id":"evt_1","user_id":"1"}}
{"topic":"events","partition":0,"offset":12346,"key":"user-2","timestamp":"2024-01-15T15:00:01Z","message_type":"mypackage.EventMessage","value":{"event_id":"evt_2","user_id":"2"}}
```

## How It Works

1. **Proto Loading**: 
   - **buf.yaml mode**: Validates the provided `buf.yaml` file exists and executes `buf build` command to compile all protos with dependencies (**Security Note**: This involves spawning an external `buf` process)
   - **Buf image/descriptor set mode**: Directly loads pre-compiled buf images or protobuf descriptor sets (`.desc`, `.pb`, `.protoset` files) - **NO external commands executed**, making it safer for production/restricted environments
   - Automatically detects input type based on file extension and content

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
