package decoder

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

func NewDecoder(protoDir string, messageType string) (*Decoder, error) {
	d := &Decoder{
		messageTypes: make(map[string]protoreflect.MessageType),
		registry:     new(protoregistry.Files),
		defaultType:  messageType,
	}

	if err := d.loadProtos(protoDir); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Decoder) loadProtos(protoDir string) error {
	// Check for buf.yaml
	if d.hasBufYaml(protoDir) {
		return d.loadWithBuf(protoDir)
	}
	return d.loadWithProtoc(protoDir)
}

func (d *Decoder) hasBufYaml(dir string) bool {
	checkDirs := []string{
		dir,
		filepath.Dir(dir),
		filepath.Dir(filepath.Dir(dir)),
	}

	for _, checkDir := range checkDirs {
		if _, err := os.Stat(filepath.Join(checkDir, "buf.yaml")); err == nil {
			return true
		}
	}
	return false
}

func (d *Decoder) loadWithBuf(protoDir string) error {
	tempDir := filepath.Join(os.TempDir(), "buf-kcat-descriptors")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	descriptorFile := filepath.Join(tempDir, "image.bin")

	// Find the directory with buf.yaml
	bufDir := protoDir
	for _, dir := range []string{protoDir, filepath.Dir(protoDir), filepath.Dir(filepath.Dir(protoDir))} {
		if _, err := os.Stat(filepath.Join(dir, "buf.yaml")); err == nil {
			bufDir = dir
			break
		}
	}

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

func (d *Decoder) loadWithProtoc(protoDir string) error {
	tempDir := filepath.Join(os.TempDir(), "buf-kcat-descriptors")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	// Find all proto files
	var protoFiles []string
	err := filepath.Walk(protoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".proto") {
			protoFiles = append(protoFiles, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(protoFiles) == 0 {
		return fmt.Errorf("no .proto files found in %s", protoDir)
	}

	// Compile with protoc
	descriptorFile := filepath.Join(tempDir, "descriptor.pb")
	args := []string{
		"--descriptor_set_out=" + descriptorFile,
		"--include_imports",
		"--include_source_info",
		"--proto_path=" + protoDir,
	}
	
	// Add parent directories as proto paths
	if parent := filepath.Dir(protoDir); parent != "." && parent != "/" {
		args = append(args, "--proto_path="+parent)
	}
	
	args = append(args, protoFiles...)
	
	cmd := exec.Command("protoc", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	
	if err := cmd.Run(); err != nil {
		// Try individual files if batch fails
		var lastErr error
		for _, protoFile := range protoFiles {
			singleArgs := []string{
				"--descriptor_set_out=" + descriptorFile,
				"--include_imports",
				"--proto_path=" + protoDir,
				protoFile,
			}
			if err := exec.Command("protoc", singleArgs...).Run(); err == nil {
				if data, err := os.ReadFile(descriptorFile); err == nil {
					d.loadDescriptorSet(data)
				}
			} else {
				lastErr = err
			}
		}
		if len(d.messageTypes) == 0 && lastErr != nil {
			return fmt.Errorf("protoc failed: %v", lastErr)
		}
	} else {
		// Load the compiled descriptor
		data, err := os.ReadFile(descriptorFile)
		if err != nil {
			return fmt.Errorf("failed to read descriptor: %w", err)
		}
		return d.loadDescriptorSet(data)
	}

	return nil
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
			d.registry.RegisterFile(fd)
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