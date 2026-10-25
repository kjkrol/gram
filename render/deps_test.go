package render

import (
	"os/exec"
	"strings"
	"testing"
)

// TestRender_DependsOnNoECS keeps render and its tests clear of goke: drawing reads what is
// published to it, never the world.
func TestRender_DependsOnNoECS(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-test",
		"github.com/kjkrol/gram/render/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, p := range strings.Fields(string(out)) {
		if strings.HasPrefix(p, "github.com/kjkrol/goke") {
			t.Errorf("render depends on %s", p)
		}
	}
}
