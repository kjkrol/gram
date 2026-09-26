package main

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
)

func TestIslandLayout_IsGroundInASeaWithTheStopsOnIt(t *testing.T) {
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
	for k, n := range count {
		if k != "earth" && k != "sand" && k != "rock" {
			t.Errorf("%d cells of %q on the island, want earth, sand and rock alone", n, k)
		}
	}
	if count["earth"] < len(layout.Cells)/2 || count["sand"] < 50 || count["rock"] < 100 {
		t.Errorf("the island's ground %v, want earth mostly, beaches and rocky heights", count)
	}
	if len(stops) != UnitCount {
		t.Errorf("%d stops, want %d", len(stops), UnitCount)
	}
	for _, s := range stops {
		if k := kinds[s]; k != "earth" && k != "sand" {
			t.Errorf("stop %v is %q, want earth or sand", s, k)
		}
	}
	if island := len(layout.Cells); island < GridWidth*GridHeight/3 || island > GridWidth*GridHeight*2/3 {
		t.Errorf("the island covers %d of %d cells, want a third to two thirds", island, GridWidth*GridHeight)
	}
}

// The sea stays level at 0 up to the shore, the land stands at least landHeight above it, and in
// places a cliff rises straight from the sea.
func TestIslandLayout_TheLandStandsAboveTheSeaAndCliffsRiseFromIt(t *testing.T) {
	grid := board.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, _ := islandLayout(grid)
	land := map[board.CellID]bool{}
	for _, e := range layout.Cells {
		land[e.Cell] = true
	}
	cliffs := 0
	for y := 1; y < GridHeight; y++ {
		for x := 1; x < GridWidth; x++ {
			all, any := true, false
			for _, c := range [][2]int{{x - 1, y - 1}, {x, y - 1}, {x - 1, y}, {x, y}} {
				id, _ := grid.CellIndex(uint32(c[0]), uint32(c[1]))
				all, any = all && land[id], any || land[id]
			}
			h := layout.Heights(geom.NewVec(float64(x*CellSize), float64(y*CellSize)))
			switch {
			case !all && h != 0:
				t.Fatalf("corner (%d, %d) by the sea stands at %v, want 0", x, y, h)
			case all && h < landHeight:
				t.Fatalf("corner (%d, %d) inland stands at %v, want %v or more", x, y, h, landHeight)
			}
			if any && !all {
				// the next corner inland, a cell away from this one on the shore
				for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					if layout.Heights(geom.NewVec(float64((x+d[0])*CellSize), float64((y+d[1])*CellSize))) >= landHeight+30 {
						cliffs++
						break
					}
				}
			}
		}
	}
	if cliffs < 40 {
		t.Errorf("%d corners of the shore under a cliff, want stretches of cliff", cliffs)
	}
}

// The heights make the relief: a range whose peaks stand far over the lowland, a plateau flat on
// top, and lowland in places.
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
	if low < len(layout.Cells)/10 {
		t.Errorf("%d of %d cells are lowland, want a tenth or more", low, len(layout.Cells))
	}
}

// Rock is steep or high ground, sand lies by the sea or on the lowland and is gentle.
func TestIslandLayout_PutsEachSoilWhereItBelongs(t *testing.T) {
	grid := board.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, _ := islandLayout(grid)
	for _, e := range layout.Cells {
		x, y, _ := grid.Coords(e.Cell)
		top, steep := 0.0, 0.0
		var hs []float64
		for _, d := range [4][2]uint32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			h := layout.Heights(geom.NewVec(float64((x+d[0])*CellSize), float64((y+d[1])*CellSize)))
			for _, o := range hs {
				steep = max(steep, math.Abs(h-o)/CellSize)
			}
			hs, top = append(hs, h), max(top, h)
		}
		switch e.Kind {
		case "rock":
			if steep < rockSlope && top < rockHeight && top >= landHeight+lowlandRoll {
				t.Errorf("rock at (%d, %d) on ground neither steep (%.2f) nor high (%.0f)", x, y, steep, top)
			}
		case "sand":
			if top >= landHeight+2*lowlandRoll || steep >= rockSlope {
				t.Errorf("sand at (%d, %d) %.0f high, %.2f steep: want it low and gentle", x, y, top, steep)
			}
		}
	}
}
