package hooks_test

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/vision/hooks"
)

// LogSightings writes a line the first time one entity sees another, and no more for the pair.
func TestLogSightings_WritesALineTheFirstTimeOneSeesAnother(t *testing.T) {
	var out bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(prev)
	h := &host.PairHost[vision.Sighting]{}
	if err := h.Add(hooks.LogSightings()); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		h.Dispatch(plugin.Tick{}, plugin.Marks{}, plugin.Marks{}, vision.Sighting{Self: 1, Seen: []vision.Seen{{ID: 2, Dist: 40}}})
	}
	h.Dispatch(plugin.Tick{}, plugin.Marks{}, plugin.Marks{}, vision.Sighting{Self: 2, Seen: []vision.Seen{{ID: 1, Dist: 40}}})
	if lines := strings.Count(out.String(), "\n"); lines != 2 {
		t.Errorf("wrote %q, want two lines: 1 seeing 2, then 2 seeing 1", out.String())
	}
}
