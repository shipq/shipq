package channelgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/codegen"
)

// makeMultiChannelSet returns a channel set with unidirectional, bidirectional,
// public, and backend channels for thorough golden file coverage.
func makeMultiChannelSet() []codegen.SerializedChannelInfo {
	return []codegen.SerializedChannelInfo{
		makeUnidirectionalEmailChannel(),  // unidirectional, authenticated
		makeBidirectionalChatbotChannel(), // bidirectional, authenticated
		makePublicAssistantChannel(),      // unidirectional, public
		makeBackendBillingChannel(),       // backend-only, should be excluded
	}
}

func runChannelGoldenTest(t *testing.T, name string, generate func() ([]byte, error)) {
	t.Helper()

	output, err := generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	goldenPath := filepath.Join("testdata", "golden", name)

	if *updateGolden {
		dir := filepath.Dir(goldenPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, output, 0644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Logf("updated golden file %s", goldenPath)
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file %s (run with -update to create): %v", goldenPath, err)
	}

	if string(output) != string(golden) {
		t.Errorf("output does not match golden file %s\n\nGot:\n%s\n\nWant:\n%s", goldenPath, string(output), string(golden))
	}
}

func TestGolden_ReactChannelHooks(t *testing.T) {
	channels := makeMultiChannelSet()
	runChannelGoldenTest(t, "react-shipq-channels.ts", func() ([]byte, error) {
		return GenerateReactChannelHooks(channels)
	})
}

func TestGolden_SvelteChannelHooks(t *testing.T) {
	channels := makeMultiChannelSet()
	runChannelGoldenTest(t, "svelte-shipq-channels.ts", func() ([]byte, error) {
		return GenerateSvelteChannelHooks(channels)
	})
}
