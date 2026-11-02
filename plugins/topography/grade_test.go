package topography_test

import (
	"testing"
	"time"

	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	irelief "github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
)

func TestWrite_GradePutsACellsCornersAndTheUnitOnItFollows(t *testing.T) {
	var walker kind.Of[topotest.Recruit]
	qw := topotest.NewQuasiWorld(
		t,
		false,
		func(units *board.Units[topotest.Recruit], grid grid.Grid) []kind.Entry {
			units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
			walker = units.Named("walker")
			return []kind.Entry{walker.Entry(topotest.Recruit{Start: grid.At(0, 3)})}
		},
	)
	altitude := func() float32 {
		t.Helper()
		zs := qw.Zs()[walker.ID()]
		if len(zs) != 1 {
			t.Fatalf("%d walkers, want one", len(zs))
		}
		return zs[0].Altitude
	}
	qw.ECS.Tick(time.Second / 60)
	if got := altitude(); got != 0 {
		t.Fatalf("the walker stands at %v before the grade, want 0 on the grass", got)
	}
	relief := qw.Topo.Write().Relief().(*irelief.Relief)
	c := qw.Grid.At(0, 3)
	version := relief.Version()
	// a ramp up east: the outline's top-left, top-right, bottom-right and bottom-left
	ramp := []float32{4, 8, 8, 4}
	if !qw.Topo.Write().Grade(c, ramp) {
		t.Fatal("the grade was refused with no road about")
	}
	if got := relief.Corners(c); got != (irelief.Corners{4, 8, 4, 8}) {
		t.Errorf("the cell's corners stand at %v, want 4 on the west and 8 on the east", got)
	}
	if relief.Version() == version {
		t.Error("the grade left the relief's Version as it was")
	}
	qw.ECS.Tick(time.Second / 60)
	if got := altitude(); f32.Abs(got-6) > 1e-4 {
		t.Errorf("the walker in the cell's middle stands at %v after a step, want 6, halfway up",
			got)
	}
	version = relief.Version()
	if !qw.Topo.Write().Grade(c, ramp) || relief.Version() != version {
		t.Error("graded as it stands, the cell does not report it stands so or counts a change")
	}
}

func TestWrite_GradeBreakingARoadMovesNothing(t *testing.T) {
	qw := topotest.NewQuasiWorld(
		t,
		false,
		func(*board.Units[topotest.Recruit], grid.Grid) []kind.Entry { return nil },
	)
	qw.ECS.Tick(time.Second / 60)
	g := qw.Grid
	road := cell.Kind{Name: cell.Named("road"), Pace: 1, Allows: cell.Land, Graded: true}
	east, _ := grid.Direction(g, 1, 0)
	west := g.Opposite(east)
	for x := range uint32(3) {
		qw.Board.Write().Board().SetWay(g.At(x, 3),
			cell.Way{Kind: road, Width: 6, Links: 1<<east | 1<<west})
	}
	relief := qw.Topo.Write().Relief().(*irelief.Relief)
	beside := g.At(1, 2)
	on := g.At(1, 3) // the road's, under beside
	before := relief.Corners(beside)
	under := relief.Corners(on)
	if qw.Topo.Write().Grade(beside, []float32{5, 5, 5, 5}) {
		t.Error("a grade lifting the road's side off its level was not refused")
	}
	if relief.Corners(beside) != before || relief.Corners(on) != under {
		t.Errorf("the refused grade left the cells at %v and the road at %v; want %v and %v",
			relief.Corners(beside), relief.Corners(on), before, under)
	}
}
