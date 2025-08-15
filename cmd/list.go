package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/HurSungYun/buf-kcat/internal/decoder"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [proto-directory]",
	Short: "List available message types",
	Long: `List all protobuf message types found in the proto directory.
	
Examples:
  # List using flag
  buf-kcat list -p /path/to/protos
  
  # List using positional argument
  buf-kcat list /path/to/protos`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Get proto directory from positional arg or flag
		if len(args) > 0 {
			protoDir = args[0]
		} else {
			protoDir, _ = cmd.Flags().GetString("proto")
			if protoDir == "" {
				protoDir = "."
			}
		}
		dec, err := decoder.NewDecoder(protoDir, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load protos: %v\n", err)
			os.Exit(1)
		}

		types := dec.MessageTypes()
		sort.Strings(types)

		fmt.Printf("Found %d message types:\n", len(types))
		for _, t := range types {
			fmt.Printf("  %s\n", t)
		}
	},
}
