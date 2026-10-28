package board_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/world"
)

func expectPanic(t *testing.T, want string, run func()) {
	t.Helper()
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, want) {
			t.Errorf("panic %q, want one mentioning %q", msg, want)
		}
	}()
	run()
	t.Errorf("no panic, want one mentioning %q", want)
}

func TestSimpleMap_IsFlatAndPricesNothingBeyondTheKinds(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 20}, Heights: true})
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewPlugin(grid, &cell.MultipleOccupancy{}, w)
	m := brd.Read(0).Board().Map()
	a := grid.At(0, 0)
	b := grid.At(1, 0)
	if corners, level := m.Top(a); corners != [4]float32{} || level != 0 {
		t.Errorf("a simple map's tile stands at %v, %v; want level ground at 0", corners, level)
	}
	if m.Rise(a, b) != 0 {
		t.Error("a simple map says a step is steep")
	}
	if m.Look() != look.Flat || m.Dressing() == nil {
		t.Error("a simple map is not seen flat from above with its bands over the tiles")
	}
}
