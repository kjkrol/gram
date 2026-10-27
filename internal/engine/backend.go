package engine

import (
	"strings"

	"github.com/gogpu/gputypes"
)

// backendOf is the backend gogpu names its device by ("Pure Go (Vulkan)"), Empty for one it
// names otherwise: render then hands the device its programs' WGSL, which every backend takes.
func backendOf(name string) gputypes.Backend {
	for _, b := range []gputypes.Backend{
		gputypes.BackendVulkan,
		gputypes.BackendMetal,
		gputypes.BackendDX12,
		gputypes.BackendGL,
	} {
		if strings.Contains(name, "("+b.String()+")") {
			return b
		}
	}
	return gputypes.BackendEmpty
}
