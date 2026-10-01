package hooks_test

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/hooks"
	"github.com/kjkrol/gram/plugins/board/unit"
)

// LogFalls writes one line for an entity standing where it may not, however long it stands there,
// and none for one on ground that takes it.
func TestLogFalls_WritesALineOnceForEachWhoFell(t *testing.T) {
	var out bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(prev)
	h := &host.ListHost[unit.Standing]{}
	if err := h.Add(hooks.LogFalls()); err != nil {
		t.Fatal(err)
	}
	hole := cell.Kind{Name: cell.Named("hole")}
	grass := cell.Kind{Name: cell.Named("grass"), Allows: cell.Land}
	for range 3 {
		h.Run(plugin.Tick{}, unit.Standing{ID: 1, Cell: 4, Kind: hole, Domain: cell.Land})
		h.Run(plugin.Tick{}, unit.Standing{ID: 2, Cell: 5, Kind: grass, Domain: cell.Land})
	}
	if lines := strings.Count(out.String(), "\n"); lines != 1 || !strings.Contains(out.String(), "entity 1 fell into the hole at cell 4") {
		t.Errorf("wrote %q, want one line of entity 1 falling into the hole", out.String())
	}
}
