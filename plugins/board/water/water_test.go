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

var cfg = water.Config{BrookAt: 4, StreamAt: 8, RiverAt: 30, BrookDepth: 1, StreamDepth: 2, RiverDepth: 4, FordEvery: 4, FordSlope: 0.5, WidthPerRoot: 1}

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

// A valley running slantwise gathers a river running slantwise: its water goes down across the
// corners of its cells, not round them in steps.
func TestDrain_ARiverRunsSlantwiseDownASlantingValley(t *testing.T) {
	grid := board.DefaultGrids{}.Square(20, 20, size)
	heights := func(p geom.Vec) float64 {
		if p.X+p.Y >= 34*size {
			return 0
		}
		return 8 + 0.4*(34*size-p.X-p.Y) + 0.6*math.Abs(p.X-p.Y)
	}
	sea := func(c board.CellID) bool { x, y, _ := grid.Coords(c); return x+y >= 33 }
	n, err := water.Drain(grid, heights, sea, cfg)
	if err != nil {
		t.Fatal(err)
	}
	slant, rivers := 0, 0
	for c, k := range n.Courses {
		if k != water.River && k != water.Ford {
			continue
		}
		rivers++
		x, y, _ := grid.Coords(c)
		dx, dy, _ := grid.Coords(n.Down[c])
		if dx != x && dy != y {
			slant++
		}
	}
	if rivers == 0 || slant*2 < rivers {
		t.Errorf("%d of %d river cells run slantwise, want most down the slanting valley", slant, rivers)
	}
}

// A course links down to where its water goes and up to each course draining into it, and runs
// the wider the more water it gathers, a cell wide at most.
func TestNetwork_LinksACourseUpAndDownAndWidensItWithItsWater(t *testing.T) {
	grid, heights, sea := valley()
	n, _ := water.Drain(grid, heights, sea, cfg)
	kinds := map[water.Course]int{}
	for c, k := range n.Courses {
		kinds[k]++
		links := n.Links(grid, c)
		if l, _ := board.Link(grid, c, n.Down[c]); links&l == 0 {
			t.Errorf("course %v does not link down to %v", c, n.Down[c])
		}
		for m, km := range n.Courses {
			if km != water.Dry && n.Down[m] == c {
				if l, _ := board.Link(grid, c, m); links&l == 0 {
					t.Errorf("course %v does not link up to %v draining into it", c, m)
				}
			}
		}
		if w := n.Width(c, size); w <= 0 || w > size || w != min(math.Sqrt(n.Gathered[c]), size) {
			t.Errorf("course %v gathering %v runs %v wide", c, n.Gathered[c], w)
		}
	}
	if kinds[water.Brook] == 0 || kinds[water.Stream] == 0 || kinds[water.River]+kinds[water.Ford] == 0 {
		t.Errorf("courses %v, want brooks, streams and a river", kinds)
	}
	var dry board.CellID
	grid.EachCell(func(c board.CellID) {
		if _, wet := n.Courses[c]; !wet && !sea(c) {
			dry = c
		}
	})
	if n.Links(grid, dry) != 0 || n.Width(dry, size) != 0 {
		t.Error("a dry cell links on or has a width")
	}
}

// A course reaching the sea runs on out into it, the way of its last step, each cell wider and more
// faded than the last, linked on from cell to cell; none without a Plume.
func TestDrain_ACourseRunsOnOutIntoTheSea(t *testing.T) {
	grid, heights, sea := valley()
	plume := cfg
	plume.Plume = 0.5
	n, _ := water.Drain(grid, heights, sea, plume)
	var mouths []board.CellID
	for c, k := range n.Courses {
		if k == water.Mouth {
			mouths = append(mouths, c)
			if !sea(c) {
				t.Errorf("a mouth at %v, ashore", c)
			}
		}
	}
	if len(mouths) < 2 {
		t.Fatalf("%d cells of mouth out at sea, want the river running on out", len(mouths))
	}
	for _, c := range mouths {
		up := board.CellID(0)
		for m, k := range n.Courses {
			if k != water.Dry && n.Down[m] == c {
				up = m
			}
		}
		if n.Width(c, size) < n.Width(up, size) || n.Fade(c) <= n.Fade(up) {
			t.Errorf("mouth %v: %v wide, faded %v; the cell before it %v wide, faded %v — want it wider and more faded",
				c, n.Width(c, size), n.Fade(c), n.Width(up, size), n.Fade(up))
		}
		if l, _ := board.Link(grid, c, up); n.Links(grid, c)&l == 0 {
			t.Errorf("mouth %v does not link back to %v", c, up)
		}
	}
	n, _ = water.Drain(grid, heights, sea, cfg)
	for _, k := range n.Courses {
		if k == water.Mouth {
			t.Fatal("a mouth out at sea with no Plume")
		}
	}
}

// Along a course the way down grows from 0 at its head to 1 at its last cell ashore, never falling
// downstream.
func TestNetwork_AlongGrowsDownACourseToTheSea(t *testing.T) {
	grid, heights, sea := valley()
	n, err := water.Drain(grid, heights, sea, cfg)
	if err != nil {
		t.Fatal(err)
	}
	heads, ends := 0, 0
	for c, k := range n.Courses {
		if k == water.Dry {
			continue
		}
		a := n.Along(c)
		if a < 0 || a > 1 {
			t.Fatalf("along %v at %v, want 0 to 1", a, c)
		}
		if a == 0 {
			heads++
		}
		d := n.Down[c]
		if sea(d) {
			ends++
			if a != 1 {
				t.Errorf("the last cell ashore at %v is %v along, want 1", c, a)
			}
		} else if n.Courses[d] != water.Dry && n.Along(d) < a {
			t.Errorf("along falls from %v at %v to %v below it", a, c, n.Along(d))
		}
	}
	if heads == 0 || ends == 0 {
		t.Errorf("%d heads, %d ends; want courses starting and reaching the sea", heads, ends)
	}
}
