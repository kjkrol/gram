package board

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world"
)

// On a ramp rising 1 in 2 eastward a step is priced by the slope of the cell it enters, the way
// it goes: up, down, across and slantwise.
func TestClimb_ReadsTheSlopeOffTheCorners(t *testing.T) {
	grid := DefaultGrids{}.Square(3, 3, 10)
	brd := NewBoard(grid, NewTerrainMap())
	brd.SetHeights(func(p geom.Vec) float64 { return 0.5 * p.X })
	at := func(x, y uint32) CellID { c, _ := grid.CellIndex(x, y); return c }
	mid := at(1, 1)
	for _, c := range []struct {
		name string
		to   CellID
		d    Domain
		want float64
	}{
		{"up", at(2, 1), Land, 1 + 10*0.5},
		{"down", at(0, 1), Land, 1 - 0.3*0.5},
		{"across", at(1, 2), Land, 1},
		{"slantwise up", at(2, 2), Land, 1 + 10*5/math.Sqrt(200)},
		{"flying up", at(2, 1), Air, 1},
		{"up on foot or wing", at(2, 1), Land | Air, 1},
	} {
		if got := brd.Climb(mid, c.to, c.d); math.Abs(got-c.want) > 1e-6 {
			t.Errorf("%s: climb %v, want %v", c.name, got, c.want)
		}
	}
}

// Raising one cell raises its neighbours' corners where they meet it.
func TestSetRelief_LeavesNoVerticalWall(t *testing.T) {
	grid := DefaultGrids{}.Square(3, 3, 10)
	brd := NewBoard(grid, NewTerrainMap())
	at := func(x, y uint32) CellID { c, _ := grid.CellIndex(x, y); return c }
	brd.SetRelief(at(1, 1), Relief{Corners: [4]float32{1, 2, 3, 4}})
	for _, c := range []struct {
		cell CellID
		want [4]float32
	}{
		{at(0, 0), [4]float32{0, 0, 0, 1}},
		{at(1, 0), [4]float32{0, 0, 1, 2}},
		{at(2, 1), [4]float32{2, 0, 4, 0}},
		{at(2, 2), [4]float32{4, 0, 0, 0}},
	} {
		if got := brd.Relief(c.cell).Corners; got != c.want {
			t.Errorf("cell %v's corners %v, want %v", c.cell, got, c.want)
		}
	}
}

func TestClimbing_ADescentIsNeverQuickerThanLeast(t *testing.T) {
	c := DefaultClimbing
	if got := c.Factor(-5); got != c.Least() || got != 0.7 {
		t.Errorf("down a cliff %v, least %v, want both 0.7", got, c.Least())
	}
}

// Up a ramp rising 1 in 5 a walker goes at a third of its speed, down it a little quicker, and a flyer over it
// as on the flat.
func TestTerrainSpeed_SlowsAClimbAndSpeedsADescent(t *testing.T) {
	grid := DefaultGrids{}.Square(4, 1, 10)
	terrain := NewTerrainMap()
	terrain.SetAll(CellKind{Cost: 1, Allows: Land | Air})
	brd := NewBoard(grid, terrain)
	brd.quasi3D = true
	brd.SetHeights(func(p geom.Vec) float64 { return 0.2 * p.X })
	at := func(x uint32) CellID { c, _ := grid.CellIndex(x, 0); return c }
	east, west := geom.NewVec(1, 0), geom.NewVec(-1, 0)
	walkers := []struct {
		cell uint32
		dir  geom.Vec
		d    Domain
		want float64
	}{
		{1, east, Land, 1 / (1 + 10*0.2)},
		{2, west, Land, 1 / (1 - 0.3*0.2)},
		{3, east, Air, 1},
	}
	got := speeds(t, brd, func(si *goke.SysInit, base *goke.Comp[world.Base]) {
		var mover goke.Comp[Mover]
		f := si.NewFactory(base, &mover)
		f.Create(len(walkers))
		k := 0
		for f.Next() {
			for i := range f.Cursor.IDs {
				w := walkers[k]
				base.Slice(&f.Cursor)[i] = world.Base{Pos: world.Position{AABB: CellAABB(grid, at(w.cell), 4)}, Vel: world.Velocity{Dir: w.dir, Value: 1}}
				mover.Slice(&f.Cursor)[i] = Mover{Domain: w.d}
				k++
			}
		}
	})
	for _, w := range walkers {
		if s := got[CellAABB(grid, at(w.cell), 4).TopLeft.X]; math.Abs(s-w.want) > 1e-9 {
			t.Errorf("cell %d going %v in %v: speed %v, want %v", w.cell, w.dir, w.d, s, w.want)
		}
	}
}
