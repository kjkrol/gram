package sector_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world/sector"
)

var bands = []sector.Band{
	{Reach: 100},
	{Reach: 300, Pace: 4},
}

func TestGrid_OfHoldsAPointToTheWorld(t *testing.T) {
	g := sector.NewGrid(1000, 400, false, false, 256, bands)
	if g.Len() != 8 {
		t.Fatalf("%d sectors over 1000x400 a side 256, want 4x2", g.Len())
	}
	for _, c := range []struct {
		x, y float32
		want int
	}{
		{10, 10, 0},
		{300, 10, 1},
		{999, 399, 7},
		{-50, -50, 0},
		{2000, 300, 7},
	} {
		if got := g.Of(c.x, c.y); got != c.want {
			t.Errorf("Of(%v, %v) = %d, want %d", c.x, c.y, got, c.want)
		}
	}
}

func TestGrid_BoxCutsTheLastAtTheWorldsEdge(t *testing.T) {
	g := sector.NewGrid(1000, 400, false, false, 256, bands)
	want := geom.NewAABB(geom.NewVec(768, 256), geom.NewVec(1000, 400))
	if got := g.Box(7); got != want {
		t.Errorf("the last sector %v, want %v", got, want)
	}
}

func TestGrid_PlaceLaysTheSectorsByTheNearestAnchor(t *testing.T) {
	g := sector.NewGrid(2560, 256, false, false, 256, bands)
	if g.Band(0) != g.Beyond() {
		t.Fatalf("a sector begins in band %d, want beyond", g.Band(0))
	}
	moved := 0
	g.Place([]geom.Vec{geom.NewVec(50, 100)}, 0, func(int, int, int) { moved++ })
	// sector 1 begins 206 from the anchor, sector 2 462 and beyond 300
	want := []int{0, 1, 2, 2, 2, 2, 2, 2, 2, 2}
	for i, b := range want {
		if g.Band(i) != b {
			t.Errorf("sector %d in band %d, want %d", i, g.Band(i), b)
		}
	}
	if moved != 2 {
		t.Errorf("%d sectors heard of, want the two left beyond", moved)
	}
	if got := g.Count(nil); got[0] != 1 || got[1] != 1 || got[2] != 8 {
		t.Errorf("counts %v, want 1, 1 and 8", got)
	}
}

func TestGrid_PlaceMovesAwayOnlyPastTheMargin(t *testing.T) {
	g := sector.NewGrid(2560, 256, false, false, 256, bands)
	g.Place([]geom.Vec{geom.NewVec(250, 100)}, 0, func(int, int, int) {})
	if g.Band(1) != 0 {
		t.Fatalf("the sector beside the anchor in band %d, want the first", g.Band(1))
	}
	// 106 from sector 1: past the first band's reach, short of it and the margin
	g.Place([]geom.Vec{geom.NewVec(150, 100)}, 50, func(int, int, int) {})
	if g.Band(1) != 0 {
		t.Errorf("the sector 106 away in band %d, want still the first", g.Band(1))
	}
	g.Place([]geom.Vec{geom.NewVec(100, 100)}, 50, func(int, int, int) {})
	if g.Band(1) != 1 {
		t.Errorf("the sector 156 away in band %d, want the second", g.Band(1))
	}
	g.Place([]geom.Vec{geom.NewVec(160, 100)}, 50, func(int, int, int) {})
	if g.Band(1) != 0 {
		t.Errorf("the sector 96 away in band %d, want the first at once", g.Band(1))
	}
}

func TestGrid_PlaceReachesAcrossASeam(t *testing.T) {
	g := sector.NewGrid(2560, 256, true, false, 256, bands)
	g.Place([]geom.Vec{geom.NewVec(20, 100)}, 0, func(int, int, int) {})
	if g.Band(9) != 0 {
		t.Errorf("the last sector, 20 away across the seam, in band %d, want the first", g.Band(9))
	}
	open := sector.NewGrid(2560, 256, false, false, 256, bands)
	open.Place([]geom.Vec{geom.NewVec(20, 100)}, 0, func(int, int, int) {})
	if open.Band(9) != open.Beyond() {
		t.Errorf("the last sector of an open world in band %d, want beyond", open.Band(9))
	}
}

func TestGrid_PlaceWithoutAnchorsLaysEverySectorInTheFirst(t *testing.T) {
	g := sector.NewGrid(2560, 256, false, false, 256, bands)
	g.Place(nil, 0, func(int, int, int) {})
	if got := g.Count(nil); got[0] != g.Len() {
		t.Errorf("counts %v with no anchors, want every sector in the first band", got)
	}
}
