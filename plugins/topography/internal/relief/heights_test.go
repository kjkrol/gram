package relief_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	irelief "github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
	"github.com/kjkrol/gram/plugins/world/steering"
)

// The heights ride on the topography's entities, a run of them each — what a save carries — and
// the runs follow the ground as it is shaped.
func TestHeights_LiveOnTheTopographysEntities(t *testing.T) {
	qw := topotest.NewQuasiWorld(t, false, func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
		units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
		k := units.Named("walker")
		start, _ := grid.CellIndex(0, 3)
		return []kind.Entry{k.Entry(topotest.Recruit{Start: start})}
	})
	qw.ECS.Tick(time.Second / 60)
	runs := runsOf(qw)
	if len(runs) != 1 || runs[0].First != 0 || runs[0].Count != 25 {
		t.Fatalf("runs %d, the first from %d of %d; want one of 25: a 4x4 grid's 5x5 corners", len(runs), runs[0].First, runs[0].Count)
	}
	c, _ := qw.Grid.CellIndex(0, 0)
	qw.Topo.Relief().(*irelief.Relief).SetCorners(c, irelief.Corners{7, 7, 7, 7})
	qw.ECS.Tick(time.Second / 60)
	if got := runsOf(qw)[0].Values[0]; got != 7 {
		t.Errorf("after the ground was shaped the run holds %v at the first corner, want 7", got)
	}
}

// runsOf lists the runs of heights the topography's entities carry.
func runsOf(qw *topotest.QuasiWorld) []irelief.Heights {
	var comp goke.Comp[irelief.Heights]
	var q *goke.Query
	qw.ECS.RegSys(goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&comp).Build() }})
	var out []irelief.Heights
	for q.All(); q.Next(); {
		out = append(out, comp.Slice(q.Cursor())...)
	}
	return out
}
