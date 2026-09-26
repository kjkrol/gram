package water_test

import (
	"errors"
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/water"
)

const size = 10

// valley is a slope falling south into a sea along the bottom rows, a valley down its middle and a
// hollow halfway down it.
func valley() (board.Grid, func(geom.Vec) float64, func(board.CellID) bool) {
	grid := board.DefaultGrids{}.Square(12, 24, size)
	heights := func(p geom.Vec) float64 {
		if p.Y >= 20*size {
			return 0
		}
		h := 8 + 0.4*(20*size-p.Y) + 0.3*math.Abs(p.X-6*size)
		if d := math.Hypot(p.X-6*size, p.Y-10*size); d < 2*size {
			h -= 10 * (1 - d/(2*size)) // the hollow
		}
		return h
	}
	sea := func(c board.CellID) bool { _, y, _ := grid.Coords(c); return y >= 20 }
	return grid, heights, sea
}

var cfg = water.Config{StreamAt: 8, RiverAt: 30, WideAt: 1e9, StreamDepth: 2, RiverDepth: 4, FordEvery: 4, FordSlope: 0.5}

// Every cell of land drains to the sea, the hollow's too, and the valley gathers a river.
func TestDrain_EveryCellDrainsToTheSeaAndTheValleyGathersARiver(t *testing.T) {
	grid, heights, sea := valley()
	n, err := water.Drain(grid, heights, sea, cfg)
	if err != nil {
		t.Fatal(err)
	}
	grid.EachCell(func(c board.CellID) {
		if sea(c) {
			return
		}
		at := c
		for range 1000 {
			if sea(at) {
				return
			}
			at = n.Down[at]
		}
		t.Fatalf("cell %v never reaches the sea", c)
	})
	rivers := 0
	for c, k := range n.Courses {
		if k != water.River && k != water.Ford {
			continue
		}
		rivers++
		if x, _, _ := grid.Coords(c); x < 5 || x > 6 {
			t.Errorf("a river at column %d, off the valley's floor", x)
		}
	}
	if rivers < 10 {
		t.Errorf("%d cells of river, want one down the valley", rivers)
	}
}

// The carved bed never rises downstream, the sea's corners stay at 0, and a corner of a course
// lies below the ground it was cut from.
func TestCarved_CutsABedFallingToTheSea(t *testing.T) {
	grid, heights, sea := valley()
	n, _ := water.Drain(grid, heights, sea, cfg)
	carved := n.Carved(heights)
	level := func(c board.CellID) float64 {
		x, y, _ := grid.Coords(c)
		sum := 0.0
		for _, d := range [4][2]float64{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			sum += carved(geom.NewVec((float64(x)+d[0])*size, (float64(y)+d[1])*size))
		}
		return sum / 4
	}
	cut := false
	for c, k := range n.Courses {
		if k == water.Dry {
			continue
		}
		if d := n.Down[c]; !sea(d) && level(d) > level(c)+1e-9 {
			t.Errorf("the bed rises from %v at %v to %v downstream", level(c), c, level(d))
		}
		x, y, _ := grid.Coords(c)
		p := geom.NewVec(float64(x)*size, float64(y)*size)
		cut = cut || carved(p) < heights(p)
	}
	if !cut {
		t.Error("no course cut below the ground")
	}
	if h := carved(geom.NewVec(6*size, 22*size)); h != 0 {
		t.Errorf("the sea's corner carved to %v, want 0", h)
	}
}

// Fords cross the river every FordEvery cells from its mouth, only where it runs gently.
func TestDrain_FordsCrossTheRiverWhereItIsGentle(t *testing.T) {
	grid, heights, sea := valley()
	n, _ := water.Drain(grid, heights, sea, cfg)
	fords := 0
	for _, k := range n.Courses {
		if k == water.Ford {
			fords++
		}
	}
	if fords == 0 {
		t.Error("no ford across the river")
	}
	steep := cfg
	steep.FordSlope = 0.01 // steeper than any stretch of the river
	n, _ = water.Drain(grid, heights, sea, steep)
	for _, k := range n.Courses {
		if k == water.Ford {
			t.Fatal("a ford where the river runs steeper than FordSlope")
		}
	}
}

func TestDrain_RefusesAGridOtherThanSquare(t *testing.T) {
	grid := board.DefaultGrids{}.Hex(6, 6, size)
	if _, err := water.Drain(grid, func(geom.Vec) float64 { return 1 }, func(board.CellID) bool { return false }, cfg); !errors.Is(err, water.ErrNotSquare) {
		t.Errorf("Drain over a hex grid: %v, want ErrNotSquare", err)
	}
}
