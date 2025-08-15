package decoder

import (
	"testing"
)

func TestDecoder(t *testing.T) {
	// Basic test to ensure package compiles
	// Actual decoder tests would require mocking buf CLI
	t.Run("PackageCompiles", func(t *testing.T) {
		// This test ensures the decoder package compiles
		// Full testing would require integration tests since
		// the decoder depends on external buf CLI
		t.Log("Decoder package compiles successfully")
	})
}
