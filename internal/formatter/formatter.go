package formatter

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Message struct {
	Topic       string
	Partition   int32
	Offset      int64
	Key         string
	Timestamp   time.Time
	MessageType string
	Value       interface{}
	Error       string
	RawValue    []byte
}

type Formatter interface {
	Format(msg Message) error
}

func New(format string) (Formatter, error) {
	switch format {
	case "json":
		return &JSONFormatter{indent: true}, nil
	case "json-compact":
		return &JSONFormatter{indent: false}, nil
	case "table":
		return &TableFormatter{}, nil
	case "raw":
		return &RawFormatter{}, nil
	case "pretty":
		return &PrettyFormatter{}, nil
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

type JSONFormatter struct {
	indent bool
}

func (f *JSONFormatter) Format(msg Message) error {
	output := map[string]interface{}{
		"topic":     msg.Topic,
		"partition": msg.Partition,
		"offset":    msg.Offset,
		"timestamp": msg.Timestamp.Format(time.RFC3339),
		"key":       msg.Key,
	}

	if msg.MessageType != "" {
		output["message_type"] = msg.MessageType
	}

	if msg.Error != "" {
		output["error"] = msg.Error
		output["raw_value_hex"] = hex.EncodeToString(msg.RawValue)
	} else {
		output["value"] = msg.Value
	}

	var data []byte
	var err error
	if f.indent {
		data, err = json.MarshalIndent(output, "", "  ")
	} else {
		data, err = json.Marshal(output)
	}
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

type TableFormatter struct{}

func (f *TableFormatter) Format(msg Message) error {
	fmt.Printf("================================================================================\n")
	fmt.Printf("Topic:       %s\n", msg.Topic)
	fmt.Printf("Partition:   %d\n", msg.Partition)
	fmt.Printf("Offset:      %d\n", msg.Offset)
	fmt.Printf("Timestamp:   %s\n", msg.Timestamp.Format(time.RFC3339))
	fmt.Printf("Key:         %s\n", msg.Key)
	
	if msg.MessageType != "" {
		fmt.Printf("Type:        %s\n", msg.MessageType)
	}

	if msg.Error != "" {
		fmt.Printf("Error:       %s\n", msg.Error)
		fmt.Printf("Raw (hex):   %s\n", hex.EncodeToString(msg.RawValue)[:100]+"...")
	} else {
		fmt.Printf("Value:\n")
		if jsonBytes, err := json.MarshalIndent(msg.Value, "", "  "); err == nil {
			fmt.Println(string(jsonBytes))
		} else {
			fmt.Printf("%v\n", msg.Value)
		}
	}
	
	return nil
}

type RawFormatter struct{}

func (f *RawFormatter) Format(msg Message) error {
	if msg.Error != "" {
		os.Stdout.Write(msg.RawValue)
	} else {
		if jsonBytes, err := json.Marshal(msg.Value); err == nil {
			os.Stdout.Write(jsonBytes)
		}
	}
	fmt.Println()
	return nil
}

type PrettyFormatter struct{}

func (f *PrettyFormatter) Format(msg Message) error {
	// Compact header
	header := fmt.Sprintf("[%s] %s/%d@%d", 
		msg.Timestamp.Format("15:04:05"),
		msg.Topic,
		msg.Partition,
		msg.Offset,
	)
	
	if msg.Key != "" {
		header += fmt.Sprintf(" key=%s", msg.Key)
	}
	
	if msg.MessageType != "" {
		header += fmt.Sprintf(" type=%s", shortTypeName(msg.MessageType))
	}
	
	fmt.Printf("%s%s%s\n", "\033[36m", header, "\033[0m") // Cyan header
	
	if msg.Error != "" {
		fmt.Printf("\033[31mError: %s\033[0m\n", msg.Error) // Red error
		if len(msg.RawValue) > 0 {
			fmt.Printf("Raw: %s...\n", hex.EncodeToString(msg.RawValue)[:min(100, len(msg.RawValue)*2)])
		}
	} else {
		if jsonBytes, err := json.MarshalIndent(msg.Value, "", "  "); err == nil {
			fmt.Println(string(jsonBytes))
		} else {
			fmt.Printf("%v\n", msg.Value)
		}
	}
	
	return nil
}

func shortTypeName(fullName string) string {
	parts := strings.Split(fullName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return fullName
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}