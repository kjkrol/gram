package terrain_test

import (
	"testing"
	"time"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
)

func TestCells_EveryCellCarriesTheRostersCellDefaults(t *testing.T) {
	for name, g := range cellGrids() {
		t.Run(name, func(t *testing.T) {
			_, _, _, probe := installCells(t, g, func(w *world.Plugin, _ *board.Plugin) {
				giveCells(w, comp.Load(func(c cell.ID) fuel { return fuel{Left: int(c) + 1} }))
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

func TestCells_ATemplateRefusedPanicsSayingWhy(t *testing.T) {
	g := cellGrids()["square"]
	for name, c := range map[string]struct {
		give comp.Comp
		want []string
	}{
		"the board's own":    {comp.Const(cell.Ground{}), []string{"cell.Ground", "the board gives every cell itself"}},
		"a row not the cell": {comp.Load(func(r struct{ n int }) fuel { return fuel{r.n} }), []string{"terrain_test.fuel", "a cell's row is its cell.ID"}},
	} {
		t.Run(name, func(t *testing.T) {
			panicsWith(t, func() {
				installCells(t, g, func(w *world.Plugin, _ *board.Plugin) { giveCells(w, c.give) })
			}, c.want...)
		})
	}
}

// giving is a plugin giving every cell c besides the board's own
type giving struct{ c comp.Comp }

var _ plugin.KindTemplater = giving{}

func (giving) Name() string                       { return "giving" }
func (giving) Install(plugin.Installer) error     { return nil }
func (giving) RunPlan(goke.RunCtx, time.Duration) {}
func (giving) EventHandler() control.EventHandler { return nil }
func (giving) Serializable() plugin.Serializable  { return nil }

func (g giving) KindTemplates(r *kind.Roster) { r.Default(board.CellTemplate, g.c) }

func giveCells(w *world.Plugin, c comp.Comp) { world.NewSelf(w, giving{c}) }
