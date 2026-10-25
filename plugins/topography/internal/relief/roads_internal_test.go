package relief

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

func TestRelief_KeepRoadsTakesBackWhatBreaksARoad(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 3, 32)
	brd, brdW := board.NewBoard(g)
	brdW.SetAll(cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land})
	road := cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land, Graded: true}
	east, _ := grid.Link(g, g.At(0, 1), g.At(1, 1))
	west, _ := grid.Link(g, g.At(1, 1), g.At(0, 1))
	for x := range uint32(3) {
		brdW.SetWay(g.At(x, 1), cell.Way{Kind: road, Width: 6, Links: east | west})
	}
	r := New(brd, brdW.Touch)
	lift := func(x, y float32) {
		r.KeepRoads(func() { r.Lift(geom.NewVec(x*32, y*32), 5, 0) })
	}
	lift(1, 1) // a corner under the road: one cell's ramp, the next's too, neither even
	if r.Corners(g.At(1, 1)) != (Corners{}) {
		t.Errorf("a corner under the road was raised: %v", r.Corners(g.At(1, 1)))
	}
	lift(4, 0) // far from the road
	if r.Corners(g.At(3, 0))[1] != 5 {
		t.Errorf("a corner away from the road stays %v, want raised to 5", r.Corners(g.At(3, 0)))
	}
}
