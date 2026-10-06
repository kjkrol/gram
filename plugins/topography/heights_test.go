package topography_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/topography"
	irelief "github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

// coverAcross walks the board's Cover along row 3, west to east, listing every stretch.
func coverAcross(qw *topotest.QuasiWorld) [][5]float64 {
	var out [][5]float64
	qw.Board.Cover().Walk(geom.NewVec(1, 3*32+16), geom.NewVec(1, 0), 126, 0, func(near, far, bottom, top, tau float64) bool {
		out = append(out, [5]float64{near, far, bottom, top, tau})
		return true
	})
	return out
}

// The cover of a cell spans its ground and its kind's Height over it, and follows the ground
// when it is lifted.
func TestCover_SpansTheCellsBandAndFollowsItsGround(t *testing.T) {
	qw := topotest.NewQuasiWorld(t, true, func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
		units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
		k := units.Named("walker")
		start := grid.CellIndex(0, 3)
		return []kind.Entry{k.Entry(topotest.Recruit{Start: start})}
	})
	brd := qw.Board.Res.Logic.Board
	wallCell := qw.Grid.CellIndex(3, 3)
	brd.Set(wallCell, cell.Kind{Name: cell.Named("wall"), Cost: 1, Solid: true, Veil: 1, Height: 10})
	qw.Topo.Relief().(*irelief.Relief).SetCorners(wallCell, irelief.Corners{12, 12, 12, 12})
	qw.ECS.Tick(time.Second / 60)

	if got := coverAcross(qw); len(got) != 1 || got[0] != [5]float64{95, 126, 12, 22, 0} {
		t.Errorf("cover along the row %v, want the wall from 95 on, 12 to 22, opaque", got)
	}
	qw.Topo.Relief().(*irelief.Relief).SetCorners(wallCell, irelief.Corners{20, 20, 20, 20})
	if got := coverAcross(qw); len(got) != 1 || got[0][2] != 20 || got[0][3] != 30 {
		t.Errorf("cover after lifting the wall %v, want it from 20 to 30", got)
	}
}

func TestPlugin_RefusesAFlatWorld(t *testing.T) {
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "Heights") {
			t.Errorf("NewPlugin over a flat world: %q, want a refusal naming irelief.Heights", r)
		}
	}()
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 128, Height: 128}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 20}})
	topography.NewPlugin(w, board.NewPlugin(grid.DefaultGrids{}.Square(4, 4, 32), &cell.MultipleOccupancy{}, w), topography.Config{Cell: 32})
}
