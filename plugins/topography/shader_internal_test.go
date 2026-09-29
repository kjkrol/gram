package topography

import (
	"os"
	"testing"

	"github.com/kjkrol/gram/plugins/topography/terrain"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

// needGPU readies a device without a window; a machine without one skips.
func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}

// The composer's one shader, the topography's water among its materials, compiles.
func TestShader_Compiles(t *testing.T) {
	needGPU(t)
	if err := render.Compile(); err != nil {
		t.Fatal(err)
	}
}

// The ground's mesh shader compiles on the composer's library, the topography's water among the
// materials it calls.
func TestTerrainShader_Compiles(t *testing.T) {
	needGPU(t)
	if err := terrain.Shader().Compile(); err != nil {
		t.Fatal(err)
	}
}
