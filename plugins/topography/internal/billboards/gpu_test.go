package billboards_test

import (
	"os"
	"testing"

	"github.com/kjkrol/gram/render/gpu"
)

func needGPU(t *testing.T) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}
