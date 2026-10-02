package terrain_test

import (
	"testing"

	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
)

// Every cell carries what the world's roster gives a cell: a Const the same everywhere, a Load
// read off the cell's own cell.ID.
func TestCells_EveryCellCarriesTheRostersCellDefaults(t *testing.T) {
	for name, g := range wireGrids() {
		t.Run(name, func(t *testing.T) {
			_, _, _, probe := installCells(t, g, func(w *world.Plugin, _ *board.Plugin) {
				w.Roster().Cell.Default(comp.Load(func(c cell.ID) fuel { return fuel{Left: int(c) + 1} }))
			})
			got := probe.read(t)
			if len(got) != g.CellCount() {
				t.Fatalf("%d cell entities, want %d", len(got), g.CellCount())
			}
			for c, st := range got {
				if st.fuel == nil || st.fuel.Left != int(c)+1 {
					t.Errorf("cell %d carries fuel %v, want %d", c, st.fuel, int(c)+1)
				}
			}
		})
	}
}

// What the board gives every cell itself is no template's, and a cell's row is its cell.ID: the
// cells panic as they are made, saying so.
func TestCells_ATemplateRefusedPanicsSayingWhy(t *testing.T) {
	g := wireGrids()["square"]
	for name, c := range map[string]struct {
		give comp.Comp
		want []string
	}{
		"the board's own":    {comp.Const(cell.Ground{}), []string{"cell.Ground", "the board gives every cell itself"}},
		"a row not the cell": {comp.Load(func(r struct{ n int }) fuel { return fuel{r.n} }), []string{"terrain_test.fuel", "a cell's row is its cell.ID"}},
	} {
		t.Run(name, func(t *testing.T) {
			panicsWith(t, func() {
				installCells(t, g, func(w *world.Plugin, _ *board.Plugin) { w.Roster().Cell.Default(c.give) })
			}, c.want...)
		})
	}
}
