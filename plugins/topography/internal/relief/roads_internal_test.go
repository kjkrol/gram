package relief

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

func TestRelief_KeepRoadsTakesBackWhatBreaksARoad(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 3, 32)
	brd, brdW := board.NewBoard(g)
	brdW.SetAll(cell.Kind{Name: cell.Named("grass"), Pace: 1, Allows: cell.Land})
	road := cell.Kind{Name: cell.Named("road"), Pace: 1, Allows: cell.Land, Graded: true}
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

func TestRelief_KeepRoadsTakesBackAGradeThatBreaksARoad(t *testing.T) {
	for name, g := range map[string]grid.Grid{
		"square": grid.DefaultGrids{}.Square(5, 3, 32),
		"hex":    grid.DefaultGrids{}.Hex(6, 4, 16),
	} {
		t.Run(name, func(t *testing.T) {
			brd, brdW := board.NewBoard(g)
			brdW.SetAll(cell.Kind{Name: cell.Named("grass"), Pace: 1, Allows: cell.Land})
			road := cell.Kind{Name: cell.Named("road"), Pace: 1, Allows: cell.Land, Graded: true}
			east, _ := grid.Direction(g, 1, 0)
			west := g.Opposite(east)
			for x := range uint32(3) {
				brdW.SetWay(g.At(x, 1), cell.Way{Kind: road, Width: 6, Links: 1<<east | 1<<west})
			}
			r := New(brd, brdW.Touch)
			raised := func(c cell.ID) []float32 {
				return slices.Repeat([]float32{5}, len(g.CellOutline(c, nil)))
			}
			before := slices.Clone(r.values)
			over := g.At(1, 0) // its lower corners the road's upper ones
			if r.KeepRoads(func() { r.Grade(over, raised(over), 0) }) {
				t.Error("a grade lifting the road's side off its level was kept")
			}
			if !slices.Equal(r.values, before) {
				t.Errorf("a grade breaking the road left the heights %v; want them as they were",
					r.values)
			}
			away := g.At(4, 2)
			if !r.KeepRoads(func() { r.Grade(away, raised(away), 0) }) {
				t.Error("a grade away from the road was taken back")
			}
			vs, n, _ := r.outline(away)
			for _, v := range vs[:n] {
				if got := r.heightAt(v); got != 5 {
					t.Errorf("a corner graded away from the road stands at %v, want 5", got)
				}
			}
		})
	}
}
