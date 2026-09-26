package main

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
)

func TestIslandLayout_IsLandInASeaWithTheStopsOnIt(t *testing.T) {
	grid := board.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, stops := islandLayout(grid)
	if layout.Default != "water" {
		t.Errorf("default kind %q, want water round the island", layout.Default)
	}
	count := map[string]int{}
	kinds := map[board.CellID]string{}
	for _, e := range layout.Cells {
		count[e.Kind]++
		kinds[e.Cell] = e.Kind
	}
	if len(count) != 1 || count["land"] == 0 {
		t.Errorf("kinds on the island %v, want land alone", count)
	}
	if len(stops) != UnitCount {
		t.Errorf("%d stops, want %d", len(stops), UnitCount)
	}
	for _, s := range stops {
		if kinds[s] != "land" {
			t.Errorf("stop %v is %q, want land", s, kinds[s])
		}
	}
	if island := len(layout.Cells); island < GridWidth*GridHeight/3 || island > GridWidth*GridHeight*2/3 {
		t.Errorf("the island covers %d of %d cells, want a third to two thirds", island, GridWidth*GridHeight)
	}
}

// The land stands landHeight above the sea, sloping down at the shore; the sea stays level at 0.
func TestIslandLayout_TheLandStandsAboveTheSea(t *testing.T) {
	grid := board.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, _ := islandLayout(grid)
	land := map[board.CellID]bool{}
	for _, e := range layout.Cells {
		land[e.Cell] = true
	}
	corner := func(x, y int) geom.Vec { return geom.NewVec(float64(x*CellSize), float64(y*CellSize)) }
	if h := layout.Heights(corner(1, 1)); h != 0 {
		t.Errorf("the open sea stands at %v, want 0", h)
	}
	// along the row through the middle, from the western sea to the first corner all of whose cells are land
	y := GridHeight / 2
	for x := 1; x < GridWidth/2; x++ {
		all := true
		for _, c := range [][2]int{{x - 1, y - 1}, {x, y - 1}, {x - 1, y}, {x, y}} {
			id, _ := grid.CellIndex(uint32(c[0]), uint32(c[1]))
			all = all && land[id]
		}
		if h := layout.Heights(corner(x, y)); all && h != landHeight || !all && h != 0 {
			t.Fatalf("corner (%d, %d) stands at %v; want %v inland, 0 at the shore", x, y, h, landHeight)
		}
		if all {
			return
		}
	}
	t.Fatal("no land along the middle row")
}

// The heights make the relief: a range whose peaks stand far over the lowland, a plateau flat on
// top, and lowland all round.
func TestIslandLayout_RisesToARangeOfPeaksAndAPlateau(t *testing.T) {
	grid := board.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, _ := islandLayout(grid)
	top, plateau, low := 0.0, 0, 0
	for _, e := range layout.Cells {
		x, y, _ := grid.Coords(e.Cell)
		flat := true
		for _, d := range [4][2]uint32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			h := layout.Heights(geom.NewVec(float64((x+d[0])*CellSize), float64((y+d[1])*CellSize)))
			top = max(top, h)
			flat = flat && math.Abs(h-landHeight-plateauHeight) < 1e-6
		}
		if flat {
			plateau++
		}
		if h := layout.Heights(grid.CellCenter(e.Cell)); h < landHeight+lowlandRoll+2 {
			low++
		}
	}
	if top < 150 {
		t.Errorf("the highest peak stands at %v, want 150 or more", top)
	}
	if plateau < 20 {
		t.Errorf("%d cells level on the plateau, want 20 or more", plateau)
	}
	if low < len(layout.Cells)/4 {
		t.Errorf("%d of %d cells are lowland, want a quarter or more", low, len(layout.Cells))
	}
}
