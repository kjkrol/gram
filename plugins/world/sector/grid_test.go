package sector_test

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world/sector"
)

func TestGrid_OfHoldsAPointToTheWorld(t *testing.T) {
	g := sector.NewGrid(1000, 400, false, false, 256)
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
	g := sector.NewGrid(1000, 400, false, false, 256)
	want := geom.NewAABB(geom.NewVec(768, 256), geom.NewVec(1000, 400))
	if got := g.Box(7); got != want {
		t.Errorf("the last sector %v, want %v", got, want)
	}
}

func TestGrid_NearestIsTheSectorsPointNearestAndHowFar(t *testing.T) {
	g := sector.NewGrid(2560, 256, false, false, 256)
	if x, y, d := g.Nearest(2, 50, 100); x != 512 || y != 100 || d != 462 {
		t.Errorf("sector 2 from (50, 100): (%v, %v) %v off, want (512, 100) 462 off", x, y, d)
	}
	if x, y, d := g.Nearest(0, 50, 100); x != 50 || y != 100 || d != 0 {
		t.Errorf("sector 0 from within: (%v, %v) %v off, want the point itself", x, y, d)
	}
}

func TestGrid_NearestReachesAcrossASeam(t *testing.T) {
	g := sector.NewGrid(2560, 256, true, false, 256)
	if x, _, d := g.Nearest(9, 20, 100); x != 2560 || d != 20 {
		t.Errorf("the last sector from 20: at %v %v off, want its edge 20 off across the seam",
			x, d)
	}
	open := sector.NewGrid(2560, 256, false, false, 256)
	if _, _, d := open.Nearest(9, 20, 100); d != 2284 {
		t.Errorf("the last sector of an open world from 20 is %v off, want 2284", d)
	}
}

func neighbours(g *sector.Grid, i int) []int {
	var got []int
	g.Neighbours(i, func(j int) { got = append(got, j) })
	slices.Sort(got)
	return got
}

func TestGrid_NeighboursAreTheEightRoundButAtAnEdgeThatDoesNotWrap(t *testing.T) {
	open := sector.NewGrid(1024, 768, false, false, 256)
	for i, want := range map[int][]int{
		0:  {1, 4, 5},
		3:  {2, 6, 7},
		5:  {0, 1, 2, 4, 6, 8, 9, 10},
		8:  {4, 5, 9},
		11: {6, 7, 10},
	} {
		if got := neighbours(open, i); !slices.Equal(got, want) {
			t.Errorf("sector %d of an open world: %v, want %v", i, got, want)
		}
	}
	across := sector.NewGrid(1024, 768, true, false, 256)
	if got, want := neighbours(across, 0), []int{1, 3, 4, 5, 7}; !slices.Equal(got, want) {
		t.Errorf("the corner of a world wrapping across: %v, want %v", got, want)
	}
	torus := sector.NewGrid(1024, 768, true, true, 256)
	if got, want := neighbours(torus, 0), []int{1, 3, 4, 5, 7, 8, 9, 11}; !slices.Equal(got, want) {
		t.Errorf("the corner of a torus: %v, want %v", got, want)
	}
}

func TestGrid_NeighboursOfANarrowTorusComeOnceAndNeverTheSectorItself(t *testing.T) {
	narrow := sector.NewGrid(512, 256, true, true, 256)
	if got, want := neighbours(narrow, 0), []int{1}; !slices.Equal(got, want) {
		t.Errorf("two sectors round a torus: %v, want %v", got, want)
	}
	if got := neighbours(sector.NewGrid(256, 256, true, true, 256), 0); len(got) != 0 {
		t.Errorf("a torus of one sector: %v, want none", got)
	}
}
