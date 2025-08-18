package decoder

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Decoder struct {
	messageTypes map[string]protoreflect.MessageType
	registry     *protoregistry.Files
	defaultType  string
}

func NewDecoder(protoPath string, messageType string) (*Decoder, error) {
	d := &Decoder{
		messageTypes: make(map[string]protoreflect.MessageType),
		registry:     new(protoregistry.Files),
		defaultType:  messageType,
	}

	// Determine input type by file extension and content
	if err := d.loadProtoDefinitions(protoPath); err != nil {
		return nil, err
	}

	return d, nil
}

// loadProtoDefinitions loads protobuf definitions from either buf.yaml or descriptor set file
func (d *Decoder) loadProtoDefinitions(protoPath string) error {
	if _, err := os.Stat(protoPath); err != nil {
		return fmt.Errorf("proto file not found: %w", err)
	}

	// Check if it's a descriptor set file (.desc, .pb, or binary content)
	if d.isDescriptorSetFile(protoPath) {
		return d.loadFromDescriptorSet(protoPath)
	}

	// Default to buf.yaml loading
	return d.loadWithBuf(protoPath)
}

// isDescriptorSetFile checks if the file is a descriptor set based on extension and content
func (d *Decoder) isDescriptorSetFile(path string) bool {
	ext := filepath.Ext(path)

	// Check common descriptor set extensions
	if ext == ".desc" || ext == ".pb" || ext == ".protoset" {
		return true
	}

	// If no clear extension, check if it's binary protobuf content
	if ext == "" || (ext != ".yaml" && ext != ".yml") {
		// Try to read a small portion and see if it looks like a descriptor set
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}

		// Try to parse as descriptor set - if it works, it's probably a descriptor set
		var fdSet descriptorpb.FileDescriptorSet
		if proto.Unmarshal(data, &fdSet) == nil {
			return true
		}

		// Try as single file descriptor
		var fdProto descriptorpb.FileDescriptorProto
		if proto.Unmarshal(data, &fdProto) == nil {
			return true
		}
	}

	return false
}

// loadFromDescriptorSet loads protobuf definitions from a descriptor set file
func (d *Decoder) loadFromDescriptorSet(descriptorFile string) error {
	data, err := os.ReadFile(descriptorFile)
	if err != nil {
		return fmt.Errorf("failed to read descriptor set file: %w", err)
	}

	return d.loadDescriptorSet(data)
}

func (d *Decoder) loadWithBuf(bufYamlPath string) error {
	if _, err := os.Stat(bufYamlPath); err != nil {
		return fmt.Errorf("buf.yaml not found: %w", err)
	}

	tempDir := filepath.Join(os.TempDir(), "buf-kcat-descriptors")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	descriptorFile := filepath.Join(tempDir, "image.bin")

	// Use the directory containing buf.yaml
	bufDir := filepath.Dir(bufYamlPath)

	// Use buf to build the image
	cmd := exec.Command("buf", "build", "-o", descriptorFile)
	cmd.Dir = bufDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("buf build failed: %v - %s", err, stderr.String())
	}

	// Read and load the descriptor
	data, err := os.ReadFile(descriptorFile)
	if err != nil {
		return fmt.Errorf("failed to read descriptor: %w", err)
	}

	return d.loadDescriptorSet(data)
}

func (d *Decoder) loadDescriptorSet(data []byte) error {
	var fdSet descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fdSet); err != nil {
		// Try as single file descriptor
		var fdProto descriptorpb.FileDescriptorProto
		if err := proto.Unmarshal(data, &fdProto); err != nil {
			return fmt.Errorf("failed to unmarshal descriptor: %w", err)
		}
		fdSet.File = []*descriptorpb.FileDescriptorProto{&fdProto}
	}

	for _, fdProto := range fdSet.File {
		fd, err := protodesc.NewFile(fdProto, d.registry)
		if err != nil {
			// Try without registry
			fd, err = protodesc.NewFile(fdProto, nil)
			if err != nil {
				continue
			}
		}

		// Register the file
		if _, err := d.registry.FindFileByPath(fd.Path()); err != nil {
			if err := d.registry.RegisterFile(fd); err != nil {
				return fmt.Errorf("failed to register file %s: %w", fd.Path(), err)
			}
		}

		// Load all messages
		d.loadMessages(fd)
	}

	if len(d.messageTypes) == 0 {
		return fmt.Errorf("no message types loaded")
	}

	return nil
}

func (d *Decoder) loadMessages(fd protoreflect.FileDescriptor) {
	messages := fd.Messages()
	for i := 0; i < messages.Len(); i++ {
		d.loadMessage(messages.Get(i))
	}
}

func (d *Decoder) loadMessage(msg protoreflect.MessageDescriptor) {
	fullName := string(msg.FullName())
	d.messageTypes[fullName] = dynamicpb.NewMessageType(msg)

	// Load nested messages
	nested := msg.Messages()
	for i := 0; i < nested.Len(); i++ {
		d.loadMessage(nested.Get(i))
	}
}

func (d *Decoder) Decode(data []byte) ([]byte, string, error) {
	// Message type is required
	if d.defaultType == "" {
		return nil, "", fmt.Errorf("message type is required")
	}

	return d.decodeWithType(data, d.defaultType)
}

func (d *Decoder) decodeWithType(data []byte, typeName string) ([]byte, string, error) {
	msgType, ok := d.messageTypes[typeName]
	if !ok {
		return nil, "", fmt.Errorf("unknown message type: %s", typeName)
	}

	msg := msgType.New().Interface()
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal: %w", err)
	}

	// Convert to JSON
	marshaler := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: false,
		UseProtoNames:   true,
	}

	jsonData, err := marshaler.Marshal(msg)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	return jsonData, typeName, nil
}

func (d *Decoder) MessageTypeCount() int {
	return len(d.messageTypes)
}

func (d *Decoder) MessageTypes() []string {
	types := make([]string, 0, len(d.messageTypes))
	for name := range d.messageTypes {
		types = append(types, name)
	}
	return types
}

// GetMessageTypes returns the map of message types for encoding
func (d *Decoder) GetMessageTypes() map[string]protoreflect.MessageType {
	return d.messageTypes
}
