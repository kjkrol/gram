package engine

import (
	"testing"

	"github.com/gogpu/gputypes"
)

func TestBackendOf_ReadsTheBackendOutOfGogpusName(t *testing.T) {
	for name, want := range map[string]gputypes.Backend{
		"Pure Go (Vulkan)": gputypes.BackendVulkan,
		"Pure Go (Metal)":  gputypes.BackendMetal,
		"Pure Go (DX12)":   gputypes.BackendDX12,
		"Pure Go (GL)":     gputypes.BackendGL,
		"Vulkan":           gputypes.BackendEmpty,
		"":                 gputypes.BackendEmpty,
	} {
		if got := backendOf(name); got != want {
			t.Errorf("%q: %v, want %v", name, got, want)
		}
	}
}
