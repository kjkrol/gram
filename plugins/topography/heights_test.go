package topography_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/topography"
	irelief "github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
)

func coverAcross(qw *topotest.QuasiWorld) [][5]float32 {
	var out [][5]float32
	qw.Board.Read(0).Cover().Walk(geom.NewVec(1, 3*32+16), geom.NewVec(1, 0), 126, 0,
		func(near, far, bottom, top, tau float32) bool {
			out = append(out, [5]float32{near, far, bottom, top, tau})
			return true
		})
	return out
}

func TestCover_SpansTheCellsBandAndFollowsItsGround(t *testing.T) {
	qw := topotest.NewQuasiWorld(
		t,
		true,
		func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
			units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
			k := units.Named("walker")
			start := grid.At(0, 3)
			return []kind.Entry{k.Entry(topotest.Recruit{Start: start})}
		},
	)
	brdW := qw.Board.Write().Board()
	wallCell := qw.Grid.At(3, 3)
	brdW.Set(
		wallCell,
		cell.Kind{Name: cell.Named("wall"), Pace: 1, Solid: true, Veil: 1, Height: 10},
	)
	qw.Topo.Write().Relief().(*irelief.Relief).SetCorners(wallCell, irelief.Corners{12, 12, 12, 12})
	qw.ECS.Tick(time.Second / 60)

	if got := coverAcross(qw); len(got) != 1 || got[0] != [5]float32{95, 126, 12, 22, 0} {
		t.Errorf("cover along the row %v, want the wall from 95 on, 12 to 22, opaque", got)
	}
	qw.Topo.Write().Relief().(*irelief.Relief).SetCorners(wallCell, irelief.Corners{20, 20, 20, 20})
	if got := coverAcross(qw); len(got) != 1 || got[0][2] != 20 || got[0][3] != 30 {
		t.Errorf("cover after lifting the wall %v, want it from 20 to 30", got)
	}
}

func bareWorld(t *testing.T) *topotest.QuasiWorld {
	t.Helper()
	return topotest.NewQuasiWorld(
		t,
		false,
		func(*board.Units[topotest.Recruit], grid.Grid) []kind.Entry { return nil },
	)
}

// the heights the topography's entities carry, in the relief's order, n of them
func carried(qw *topotest.QuasiWorld, n int) []float32 {
	out := make([]float32, n)
	for _, h := range qw.Heights() {
		copy(out[min(int(h.First), n):], h.Values[:h.Count])
	}
	return out
}

func TestHeights_HoldAsTheTickEndsAGradeMadeInAStepOfTheSimulation(t *testing.T) {
	qw := bareWorld(t)
	qw.ECS.Tick(time.Second / 60)
	c := qw.Grid.At(0, 3)
	graded := false
	// a plugin made after the topography grading the ground in its step, as roads lays a road
	step := func(goke.RunCtx, time.Duration) {
		graded = graded || qw.Topo.Write().Grade(c, []float32{4, 8, 8, 4})
	}
	qw.Later = func(ctx goke.RunCtx, d time.Duration) {
		clock.Simulate(qw.World.Write().Clock(), ctx, d, step)
	}
	qw.ECS.Tick(time.Second / 60)
	relief := qw.Topo.Write().Relief().(*irelief.Relief)
	if !graded || relief.Corners(c) != (irelief.Corners{4, 8, 4, 8}) {
		t.Fatalf("graded %v, the cell's corners at %v; want them graded in the step, 4 on the "+
			"west and 8 on the east", graded, relief.Corners(c))
	}
	_, _, _, want, _ := relief.Lattice()
	if got := carried(qw, len(want)); !slices.Equal(got, want) {
		t.Errorf("as the tick ends the entities carry %v, want the relief graded in its step %v",
			got, want)
	}
}

func TestHeights_HoldAsTheTickEndsTheGroundRaisedInThePause(t *testing.T) {
	qw := bareWorld(t)
	if err := qw.World.Carry(qw.Topo); err != nil {
		t.Fatal(err)
	}
	qw.ECS.Tick(time.Second / 60)
	qw.World.Write().Clock().TogglePause()
	relief := qw.Topo.Write().Relief().(*irelief.Relief)
	version := relief.Version()
	// the corner the cells (0,2), (1,2), (0,3) and (1,3) share
	if !qw.World.Write().Carrier().Put(command.Nobody, topography.Raise{At: geom.NewVec(32, 96)}) {
		t.Fatal("nobody takes the Raise")
	}
	qw.ECS.Tick(time.Second / 60)
	if relief.Version() == version {
		t.Fatal("the Raise in the pause left the ground as it was")
	}
	_, _, _, want, _ := relief.Lattice()
	if got := carried(qw, len(want)); !slices.Equal(got, want) {
		t.Errorf("as the paused tick ends the entities carry %v, want the relief raised in it %v",
			got, want)
	}
}

func TestHeights_NotWrittenAnewWhileTheGroundStands(t *testing.T) {
	qw := bareWorld(t)
	qw.ECS.Tick(time.Second / 60)
	relief := qw.Topo.Write().Relief().(*irelief.Relief)
	version := relief.Version()
	runs := qw.Heights()
	if len(runs) != 1 || runs[0].First != 0 || relief.Corners(qw.Grid.At(0, 0))[0] != 0 {
		t.Fatalf("%d runs of heights, the ground at the first corner %v; want one run from the "+
			"first corner, at 0", len(runs), relief.Corners(qw.Grid.At(0, 0))[0])
	}
	// a height the relief does not hold, which the entities written anew would lose
	runs[0].Values[0] = -1
	for range 3 {
		qw.ECS.Tick(time.Second / 60)
	}
	if relief.Version() != version || qw.Heights()[0].Values[0] != -1 {
		t.Errorf("three ticks with the ground standing: the relief's version %d, the entities' "+
			"first height %v; want %d and the -1 left there, nothing written anew",
			relief.Version(), qw.Heights()[0].Values[0], version)
	}
}
