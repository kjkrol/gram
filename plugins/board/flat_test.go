package board_test

import (
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/steering"
)

func TestUnits_CarryNoZInAFlatWorld(t *testing.T) {
	bw, _ := squareWorld(t, mover{})
	bw.tick()
	var z goke.OptComp[world.Z]
	var base goke.Comp[world.Base]
	var q *goke.Query
	bw.ecs.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base).Optional(&z).Build() }})
	for q.All(); q.Next(); {
		if z.Present(q.Cursor()) {
			t.Fatal("an entity of a flat world carries a Z")
		}
	}
}

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

func TestFlatWorld_RefusesWhatStandsAtAHeight(t *testing.T) {
	flat := func() (*world.Plugin, *board.Plugin) {
		w := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 128, Height: 128},
			Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 20, MaxSize: 20},
		})
		return w, board.NewPlugin(board.DefaultGrids{}.Square(4, 4, 32), &board.MultipleOccupancy{}, w)
	}
	at := func(recruit) geom.Vec { return geom.NewVec(48, 48) }

	t.Run("a kind with a height", func(t *testing.T) {
		_, brd := flat()
		expectPanic(t, "Heights", func() { brd.CellKinds().Create(cell.Kind{Name: cell.Named("wall"), Height: 3}) })
	})
	t.Run("units with a height", func(t *testing.T) {
		_, brd := flat()
		expectPanic(t, "Heights", func() { board.NewUnits[recruit](brd, board.Shape{Size: 20, Height: 2}, at) })
	})
	t.Run("a unit with a lift", func(t *testing.T) {
		_, brd := flat()
		units := board.NewUnits[recruit](brd, board.Shape{Size: 20}, at)
		expectPanic(t, "Heights", func() { units.Define("hawk", board.Mover{Domain: cell.Air, Lift: 40}, steering.Steering{}) })
	})
	t.Run("a unit with a Z of its own", func(t *testing.T) {
		_, brd := flat()
		units := board.NewUnits[recruit](brd, board.Shape{Size: 20}, at)
		expectPanic(t, "Heights", func() {
			units.Define("tower", board.Mover{Domain: cell.Land}, steering.Steering{}, comp.Const(world.Z{Height: 3}))
		})
	})
}

// The simple map is flat and prices nothing beyond the kinds: a step and the speed are the kind's.
func TestSimpleMap_IsFlatAndPricesNothingBeyondTheKinds(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 20}})
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
	m := brd.Map()
	a, _ := grid.CellIndex(0, 0)
	b, _ := grid.CellIndex(1, 0)
	if corners, level := m.Top(a); corners != [4]float32{} || level != 0 {
		t.Errorf("a simple map's tile stands at %v, %v; want level ground at 0", corners, level)
	}
	if m.Climb(a, b, cell.Land) != 1 || m.Least(cell.Land) != 1 || m.Slope(geom.NewVec(16, 16), geom.NewVec(1, 0), cell.Land) != 1 {
		t.Error("a simple map prices a step or the speed beyond the kind's cost")
	}
	if m.Look() != board.FlatLook() || m.Dressing() == nil {
		t.Error("a simple map is not seen flat from above with its bands over the tiles")
	}
}
