package topography

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
)

// reliefOf is a level relief over a cols x rows square grid of cells size wide.
func reliefOf(cols, rows, size uint32) (*Relief, board.Grid) {
	grid := board.DefaultGrids{}.Square(cols, rows, size)
	return NewRelief(board.NewBoard(grid, board.NewTerrainMap())), grid
}

// On a ramp rising 1 in 2 eastward a step is priced by the slope of the cell it enters, the way
// it goes: up, down, across and slantwise.
func TestClimb_ReadsTheSlopeOffTheCorners(t *testing.T) {
	r, grid := reliefOf(3, 3, 10)
	r.SetHeights(func(p geom.Vec) float64 { return 0.5 * p.X })
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	mid := at(1, 1)
	for _, c := range []struct {
		name string
		to   board.CellID
		d    board.Domain
		want float64
	}{
		{"up", at(2, 1), board.Land, 1 + 10*0.5},
		{"down", at(0, 1), board.Land, 1 - 0.3 + 5*(0.5-0.1)},
		{"across", at(1, 2), board.Land, 1},
		{"slantwise up", at(2, 2), board.Land, 1 + 10*5/math.Sqrt(200)},
		{"flying up", at(2, 1), board.Air, 1},
		{"up on foot or wing", at(2, 1), board.Land | board.Air, 1},
	} {
		if got := r.Climb(mid, c.to, c.d, DefaultClimbing); math.Abs(got-c.want) > 1e-6 {
			t.Errorf("%s: climb %v, want %v", c.name, got, c.want)
		}
	}
}

// The corners are a lattice: raising one cell's raises its neighbours' where they meet it.
func TestSetCorners_LeavesNoVerticalWall(t *testing.T) {
	r, grid := reliefOf(3, 3, 10)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	r.SetCorners(at(1, 1), Corners{1, 2, 3, 4})
	for _, c := range []struct {
		cell board.CellID
		want Corners
	}{
		{at(0, 0), Corners{0, 0, 0, 1}},
		{at(1, 0), Corners{0, 0, 1, 2}},
		{at(2, 1), Corners{2, 0, 4, 0}},
		{at(2, 2), Corners{4, 0, 0, 0}},
	} {
		if got := r.Corners(c.cell); got != c.want {
			t.Errorf("cell %v's corners %v, want %v", c.cell, got, c.want)
		}
	}
}

// A climb costs the more the steeper it is; a descent is quickest at Ease and costs the more the
// steeper it is past it; no slope is quicker than Least.
func TestClimbing_CostsTheMoreTheSteeperEitherWay(t *testing.T) {
	c := DefaultClimbing
	for _, s := range []struct{ slope, want float64 }{
		{0, 1}, {0.05, 1.5}, {0.1, 2}, {0.2, 3}, {0.5, 6}, {1, 11},
		{-0.05, 0.85}, {-0.1, 0.7}, {-0.2, 1.2}, {-0.5, 2.7}, {-1, 5.2},
	} {
		if got := c.Factor(s.slope); math.Abs(got-s.want) > 1e-9 {
			t.Errorf("a slope of %v costs ×%v, want ×%v", s.slope, got, s.want)
		}
		if c.Factor(s.slope) < c.Least() {
			t.Errorf("a slope of %v costs ×%v, under the least ×%v", s.slope, c.Factor(s.slope), c.Least())
		}
	}
	if c.Least() != 0.7 {
		t.Errorf("least ×%v, want ×0.7 at a descent of Ease", c.Least())
	}
	if flat := (Climbing{Up: 10, Steep: 5}); flat.Factor(-0.1) != 1.5 || flat.Least() != 1 {
		t.Errorf("with no Ease a descent of 1 in 10 costs ×%v, least ×%v; want ×1.5 and ×1", flat.Factor(-0.1), flat.Least())
	}
}

// Up a ramp rising 1 in 5 a walker goes at a third of its speed, down it carefully, slower than on
// the flat, and a flyer over it as on the flat: what the Map's Slope tells the board's Moving
// behaviour.
func TestSlope_SlowsAClimbAndASteepDescent(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 40, Height: 10}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 4}, Quasi3D: true})
	grid := board.DefaultGrids{}.Square(4, 1, 10)
	b := board.NewPlugin(grid, &board.MultipleOccupancy{}, w)
	p := NewPlugin(w, b, Config{Cell: 10})
	p.Relief().SetHeights(func(q geom.Vec) float64 { return 0.2 * q.X })
	east, west := geom.NewVec(1, 0), geom.NewVec(-1, 0)
	for _, c := range []struct {
		x    float64
		dir  geom.Vec
		d    board.Domain
		want float64
	}{
		{15, east, board.Land, 1 + 10*0.2},
		{25, west, board.Land, 1 - 0.3 + 5*(0.2-0.1)},
		{35, east, board.Air, 1},
	} {
		if got := p.Slope(geom.NewVec(c.x, 5), c.dir, c.d); math.Abs(got-c.want) > 1e-6 {
			t.Errorf("at x %v going %v in %v: ×%v as long, want ×%v", c.x, c.dir, c.d, got, c.want)
		}
	}
	if got := b.Map().Slope(geom.NewVec(15, 5), east, board.Land); math.Abs(got-3) > 1e-6 {
		t.Errorf("the board's map prices the climb ×%v, want the topography's ×3", got)
	}
}
