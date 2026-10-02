package moments

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
)

// The pass writes one line for a unit standing where it may not, however long it stands there,
// and none for one on ground that takes it.
func TestLog_WritesALineOnceForEachWhoFell(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(2, 1, 10)
	cells := terrain.New(grid)
	at := func(x uint32) cell.ID { c, _ := grid.CellIndex(x, 0); return c }
	cells.Set(at(0), cell.Kind{Name: cell.Named("hole")})
	cells.Set(at(1), cell.Kind{Name: cell.Named("grass"), Allows: cell.Land})
	var out bytes.Buffer
	r := New(grid, cells, nil, nil)
	r.Log = log.New(&out, "", 0)

	walk(t, r, 3,
		walker{box: cellBox(grid, at(0), 4), domain: cell.Land},
		walker{box: cellBox(grid, at(1), 4), domain: cell.Land},
	)
	if lines := strings.Count(out.String(), "\n"); lines != 1 || !strings.Contains(out.String(), "fell into the hole at cell 0") {
		t.Errorf("wrote %q, want one line of a unit falling into the hole", out.String())
	}
}
