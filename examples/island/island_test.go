package island

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

func TestIslandLayout_IsGroundInASeaWithTheStopsOnIt(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, _, stops := Layout(grid)
	if layout.Default != WaterCell {
		t.Errorf("default kind %q, want water round the island", layout.Default)
	}
	count := map[string]int{}
	kinds := map[cell.ID]string{}
	for _, e := range layout.Cells {
		count[e.Kind]++
		kinds[e.Cell] = e.Kind
	}
	for k, n := range count {
		if k != EarthCell && k != SandCell && k != RockCell {
			t.Errorf("%d cells of %q on the island, want earth, sand and rock alone", n, k)
		}
	}
	if count[EarthCell] < len(layout.Cells)/2 || count[SandCell] < 30 || count[RockCell] < 100 {
		t.Errorf("the island's ground %v, want earth mostly, beaches and rocky heights", count)
	}
	if len(stops) != Stops {
		t.Errorf("%d stops, want %d", len(stops), Stops)
	}
	for _, s := range stops {
		if k := kinds[s]; k != EarthCell && k != SandCell {
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
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, heights, _ := Layout(grid)
	land, wet := map[cell.ID]bool{}, map[[2]int]bool{}
	for _, e := range layout.Cells {
		land[e.Cell] = true
	}
	for _, w := range layout.Ways {
		x, y, _ := grid.Coords(w.Cell)
		for _, d := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			wet[[2]int{int(x) + d[0], int(y) + d[1]}] = true // a channel's bed lies below the land
		}
	}
	cliffs := 0
	for y := 1; y < GridHeight; y++ {
		for x := 1; x < GridWidth; x++ {
			all, any := true, false
			for _, c := range [][2]int{{x - 1, y - 1}, {x, y - 1}, {x - 1, y}, {x, y}} {
				id := grid.CellIndex(uint32(c[0]), uint32(c[1]))
				all, any = all && land[id], any || land[id]
			}
			h := heights(geom.NewVec(float64(x*CellSize), float64(y*CellSize)))
			switch {
			case !all && h != 0:
				t.Fatalf("corner (%d, %d) by the sea stands at %v, want 0", x, y, h)
			case all && h < landHeight && !wet[[2]int{x, y}]:
				t.Fatalf("corner (%d, %d) inland stands at %v, want %v or more", x, y, h, landHeight)
			}
			if any && !all {
				// the next corner inland, a cell away from this one on the shore
				for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					if heights(geom.NewVec(float64((x+d[0])*CellSize), float64((y+d[1])*CellSize))) >= landHeight+30 {
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
// top, and gentle ground in places.
// The plateau's flat top holds a group: twenty cells or more, each level at the plateau's height and
// none of them water, nearest its middle first.
func TestPlateau_IsLevelHighGroundForAGroup(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, heights, _ := Layout(grid)
	kinds := map[cell.ID]string{}
	for _, e := range layout.Cells {
		kinds[e.Cell] = e.Kind
	}
	top := Plateau(grid)
	if len(top) < 20 {
		t.Fatalf("%d cells on the plateau's top, want 20 or more", len(top))
	}
	kinds20 := map[string]int{}
	for _, c := range top[:20] {
		x, y, _ := grid.Coords(c)
		centre := geom.NewVec((float64(x)+0.5)*CellSize, (float64(y)+0.5)*CellSize)
		if h := heights(centre); math.Abs(h-landHeight-plateauHeight) > 1 {
			t.Errorf("cell (%d, %d) stands at %.1f, want the plateau's %v", x, y, h, landHeight+plateauHeight)
		}
		kind := kinds[c]
		if kind == "" {
			kind = layout.Default
		}
		kinds20[kind]++
		if kind == WaterCell || kind == "sea" {
			t.Errorf("cell (%d, %d) on the plateau is %s", x, y, kind)
		}
	}
	t.Logf("%d cells on the top; the first twenty: %v", len(top), kinds20)
}

func TestIslandLayout_RisesToARangeOfPeaksAndAPlateau(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, heights, _ := Layout(grid)
	top, plateau, gentle := 0.0, 0, 0
	for _, e := range layout.Cells {
		x, y, _ := grid.Coords(e.Cell)
		flat, lo, hi := true, math.Inf(1), math.Inf(-1)
		for _, d := range [4][2]uint32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			h := heights(geom.NewVec(float64((x+d[0])*CellSize), float64((y+d[1])*CellSize)))
			top, lo, hi = max(top, h), min(lo, h), max(hi, h)
			flat = flat && math.Abs(h-landHeight-plateauHeight) < 1e-6
		}
		if flat {
			plateau++
		} else if (e.Kind == EarthCell || e.Kind == SandCell) && (hi-lo)/CellSize < 0.1 {
			gentle++
		}
	}
	if top < 150 {
		t.Errorf("the highest peak stands at %v, want 150 or more", top)
	}
	if plateau < 20 {
		t.Errorf("%d cells level on the plateau, want 20 or more", plateau)
	}
	if gentle < len(layout.Cells)/20 {
		t.Errorf("%d of %d cells are gentle ground, want a twentieth or more", gentle, len(layout.Cells))
	}
}

// Rock is steep or high ground, sand lies by the sea or on the lowland and is gentle.
func TestIslandLayout_PutsEachSoilWhereItBelongs(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, heights, _ := Layout(grid)
	for _, e := range layout.Cells {
		x, y, _ := grid.Coords(e.Cell)
		top, steep := 0.0, 0.0
		var hs []float64
		for _, d := range [4][2]uint32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			h := heights(geom.NewVec(float64((x+d[0])*CellSize), float64((y+d[1])*CellSize)))
			for _, o := range hs {
				steep = max(steep, math.Abs(h-o)/CellSize)
			}
			hs, top = append(hs, h), max(top, h)
		}
		switch e.Kind {
		case RockCell:
			if steep < rockSlope && top < rockHeight && top >= landHeight+lowlandRoll {
				t.Errorf("rock at (%d, %d) on ground neither steep (%.2f) nor high (%.0f)", x, y, steep, top)
			}
		case SandCell:
			if top >= landHeight+2*lowlandRoll || steep >= rockSlope {
				t.Errorf("sand at (%d, %d) %.0f high, %.2f steep: want it low and gentle", x, y, top, steep)
			}
		}
	}
}

// A walker reaches every stop from every other: streams are waded, rivers crossed at a ford.
func TestIslandLayout_EveryStopIsReachableOnFoot(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, _, stops := Layout(grid)
	walk := map[cell.ID]bool{}
	for _, e := range layout.Cells {
		walk[e.Cell] = true
	}
	for _, w := range layout.Ways {
		if _, land := walk[w.Cell]; land {
			walk[w.Cell] = w.Kind != RiverCell
		}
	}
	seen := map[cell.ID]bool{stops[0]: true}
	queue := []cell.ID{stops[0]}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		x, y, _ := grid.Coords(c)
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := int(x)+d[0], int(y)+d[1]
			if nx < 0 || nx >= GridWidth || ny < 0 || ny >= GridHeight {
				continue
			}
			if n := grid.CellIndex(uint32(nx), uint32(ny)); walk[n] && !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	for _, s := range stops {
		if !seen[s] {
			t.Errorf("stop %v cannot be walked to from stop %v", s, stops[0])
		}
	}
}

// Somewhere the running water falls: a cell of it dropping a cell's width or more across it.
func TestIslandLayout_HasAWaterfall(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, heights, _ := Layout(grid)
	for _, w := range layout.Ways {
		x, y, _ := grid.Coords(w.Cell)
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, d := range [4][2]uint32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			h := heights(geom.NewVec(float64((x+d[0])*CellSize), float64((y+d[1])*CellSize)))
			lo, hi = min(lo, h), max(hi, h)
		}
		if hi-lo >= CellSize {
			return
		}
	}
	t.Error("no waterfall: the running water nowhere drops a cell's width")
}

// Brooks, streams and rivers run across the ground, each the wider the more water it carries, each
// linked on to a neighbour.
func TestIslandLayout_RunningWaterIsBrooksStreamsAndRivers(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, _, _ := Layout(grid)
	count, width := map[string]int{}, map[string]float32{}
	for _, w := range layout.Ways {
		count[w.Kind]++
		width[w.Kind] += w.Width
		if w.Links == 0 || w.Width <= 0 || w.Width > CellSize {
			t.Errorf("a %s at %v runs %v wide, links %b", w.Kind, w.Cell, w.Width, w.Links)
		}
	}
	if count[BrookCell] < 30 || count[StreamCell] < 10 || count[RiverCell]+count[FordCell] < 5 {
		t.Fatalf("running water %v, want brooks, streams and a river", count)
	}
	brook, stream, river := width[BrookCell]/float32(count[BrookCell]), width[StreamCell]/float32(count[StreamCell]), width[RiverCell]/float32(count[RiverCell])
	if brook >= stream || stream >= river {
		t.Errorf("brooks run %v wide, streams %v, rivers %v; want each wider than the last", brook, stream, river)
	}
}

// Running water lies ashore and takes on the sea's look down its course, all of it where it
// reaches the sea (the landscape runs it on into the water, fading).
func TestIslandLayout_RiversTurnIntoTheSeaAtTheirMouths(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, _, _ := Layout(grid)
	land := map[cell.ID]bool{}
	for _, e := range layout.Cells {
		land[e.Cell] = true
	}
	mouths, heads := 0, 0
	for _, w := range layout.Ways {
		if w.Kind == RoadCell {
			continue
		}
		if !land[w.Cell] || w.Fade != 0 || w.Mix < 0 || w.Mix > 1 {
			t.Errorf("a %s at %v: ashore %v, faded %v, mixed %v; want ashore, unfaded, mixed 0 to 1", w.Kind, w.Cell, land[w.Cell], w.Fade, w.Mix)
		}
		if w.Mix == 0 {
			heads++
		}
		for i := range 8 {
			if n, ok := grid.Toward(w.Cell, i); ok && w.Links&(1<<i) != 0 && !land[n] {
				mouths++
				if w.Mix != 1 {
					t.Errorf("a %s reaching the sea at %v mixed %v, want all the sea's look", w.Kind, w.Cell, w.Mix)
				}
			}
		}
	}
	if mouths < 5 || heads < 5 {
		t.Errorf("%d mouths, %d heads; want the courses rising inland and reaching the sea", mouths, heads)
	}
}

// Roads run from every stop to the next round the hexagon over land, one network of them, round
// the rock where they can, and a bridge carries a road over every course it crosses.
func TestIslandLayout_RoadsLinkTheStopsAndBridgeTheWater(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	layout, _, stops := Layout(grid)
	soil := map[cell.ID]string{}
	for _, e := range layout.Cells {
		soil[e.Cell] = e.Kind
	}
	road := map[cell.ID]cell.Links{}
	course := map[cell.ID]cell.WayEntry{}
	for _, w := range layout.Ways {
		if w.Kind == RoadCell {
			road[w.Cell] = w.Links
		} else {
			course[w.Cell] = w
		}
	}
	for _, b := range layout.Crossings {
		if b.Kind != BridgeCell {
			t.Errorf("a %s crossing at %v, want bridges alone", b.Kind, b.Cell)
		}
		if _, over := course[b.Cell]; !over {
			t.Errorf("a bridge at %v over no water", b.Cell)
		}
		if _, both := road[b.Cell]; both {
			t.Errorf("a road and a bridge both at %v", b.Cell)
		}
		road[b.Cell] = b.Links
	}
	if len(layout.Crossings) == 0 {
		t.Error("no bridge: the roads cross no water")
	}
	rock := 0
	for c := range road {
		switch soil[c] {
		case "":
			t.Errorf("a road at %v out at sea", c)
		case RockCell:
			rock++
		}
	}
	if rock*4 > len(road) {
		t.Errorf("%d of %d cells of road on rock, want the roads round it where they can", rock, len(road))
	}
	// every stop on the one network: walk the links from the first
	seen := map[cell.ID]bool{stops[0]: true}
	for queue := []cell.ID{stops[0]}; len(queue) > 0; queue = queue[1:] {
		c := queue[0]
		for i := range 8 {
			if road[c]&(1<<i) == 0 {
				continue
			}
			if n, ok := grid.Toward(c, i); ok && !seen[n] {
				if _, on := road[n]; on {
					seen[n] = true
					queue = append(queue, n)
				}
			}
		}
	}
	for _, s := range stops {
		if !seen[s] {
			t.Errorf("the stop at %v is not on the roads", s)
		}
	}
}
