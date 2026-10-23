// Package gputest readies the GPU for a test that draws.
package gputest

import (
	"os"
	"testing"

	"github.com/kjkrol/gram/internal/gpu"
)

// Need opens the GPU without a window, a software one where GRAM_GPU is "software", or skips t
// where there is none.
func Need(t testing.TB) {
	t.Helper()
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
}
