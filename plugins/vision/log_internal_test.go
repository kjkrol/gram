package vision

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/kjkrol/uid"
)

// The log has a line the first time one entity sees another, and no more for the pair.
func TestLog_WritesALineTheFirstTimeOneSeesAnother(t *testing.T) {
	var out bytes.Buffer
	s := &ScanSystem{log: log.New(&out, "", 0)}
	for range 3 {
		s.logSightings(1, &Sighted{IDs: [MaxSeen]uid.UID64{2}, Dists: [MaxSeen]float32{40}, Count: 1})
	}
	s.logSightings(2, &Sighted{IDs: [MaxSeen]uid.UID64{1}, Dists: [MaxSeen]float32{40}, Count: 1})
	if lines := strings.Count(out.String(), "\n"); lines != 2 {
		t.Errorf("wrote %q, want two lines: 1 seeing 2, then 2 seeing 1", out.String())
	}
}
