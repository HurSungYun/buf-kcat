# buf-kcat

A Kafka client CLI tool with protobuf encoding/decoding using buf. Combines the functionality of [kafkacat/kcat](https://github.com/edenhill/kcat) with protobuf message encoding and decoding for better debugging and monitoring.

> ⚠️ **Note**: This is a debugging/development tool that requires the `buf` CLI to be installed. Currently, it uses `buf build` command internally to compile protobuf definitions, which involves executing external commands. Not recommended for production use cases where security and reliability are critical.

## Features

- 🚀 **Protobuf encoding/decoding** - Encodes and decodes Kafka messages using protobuf definitions
- ✍️ **Producer mode** - Send JSON messages that are automatically encoded to protobuf
- 📦 **buf.yaml based** - Uses buf for proto compilation with full dependency support
- 🎨 **Multiple output formats** - JSON, table, pretty, raw formats
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

buf-kcat requires a `buf.yaml` configuration file for proto compilation:

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

### Producer Mode

Produce JSON messages that are automatically encoded to protobuf:

```bash
# Produce a single message from stdin
echo '{"user_id": "123", "event_type": "LOGIN"}' | \
  buf-kcat produce -b localhost:9092 -t events -p buf.yaml -m events.UserEvent

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

```json
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

### Example Output

When you run buf-kcat, you'll see connection status messages followed by the decoded messages:

```bash
$ buf-kcat -b localhost:9092 -t events -p ./protos/buf.yaml -m events.UserEvent --follow

Connected to Kafka brokers: [localhost:9092]
Starting to consume from topic 'events' (group: buf-kcat, offset: end)
Message type: events.UserEvent
Following topic (press Ctrl+C to stop)...
Waiting for messages...

[15:04:05] events/0@12345 key=user-123 type=events.UserEvent
{
  "user_id": "123",
  "event_type": "LOGIN",
  "timestamp": "2024-01-15T15:04:05Z",
  "metadata": {
    "ip_address": "192.168.1.1",
    "user_agent": "Mozilla/5.0"
  }
}

[15:04:12] events/0@12346 key=user-456 type=events.UserEvent
{
  "user_id": "456",
  "event_type": "LOGOUT",
  "timestamp": "2024-01-15T15:04:12Z"
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
   - Validates the provided `buf.yaml` file exists
   - Executes `buf build` command to compile all protos with dependencies
   - **Security Note**: This involves spawning an external process (`buf` CLI)

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
