package sector_test

import (
	"math"
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

func TestGrid_OfIsTheSectorFlooredAndHeld(t *testing.T) {
	floored := func(v, side float32, n int) int {
		return min(max(int(math.Floor(float64(v/side))), 0), n-1)
	}
	values := []float32{-1e9, -513, -512, -511.5, -256, -1, -0.5, -1e-6, 0, 1e-6, 0.5, 255.99,
		256, 256.01, 511.999, 512, 767, 999, 999.99, 1000, 1000.01, 1023, 1024, 4096, 1e9}
	for _, wraps := range []bool{false, true} {
		for _, side := range []float32{256, 100.5, 333} {
			g := sector.NewGrid(1000, 400, wraps, wraps, side)
			cols, rows := g.Cols(), g.Rows()
			for _, x := range values {
				for _, y := range values {
					want := floored(y, side, rows)*cols + floored(x, side, cols)
					if got := g.Of(x, y); got != want {
						t.Errorf("wrapping %v, a side %v: Of(%v, %v) = %d, want %d", wraps,
							side, x, y, got, want)
					}
				}
			}
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

func TestGrid_CoverCutsARectangleBySectorOnceEach(t *testing.T) {
	box := func(x0, y0, x1, y1 float32) geom.AABB {
		return geom.NewAABB(geom.NewVec(x0, y0), geom.NewVec(x1, y1))
	}
	part := func(i int, x0, y0, x1, y1 float32) sector.Part {
		return sector.Part{Sector: i, Box: box(x0, y0, x1, y1)}
	}
	flat := sector.NewGrid(1000, 400, false, false, 256)
	torus := sector.NewGrid(1024, 512, true, true, 256)
	uneven := sector.NewGrid(1000, 400, true, false, 512)
	for name, c := range map[string]struct {
		g    *sector.Grid
		box  geom.AABB
		want []sector.Part
	}{
		"within one": {flat, box(10, 20, 30, 40), []sector.Part{part(0, 10, 20, 30, 40)}},
		"across six": {flat, box(200, 100, 600, 300), []sector.Part{
			part(0, 200, 100, 256, 256),
			part(1, 256, 100, 512, 256),
			part(2, 512, 100, 600, 256),
			part(4, 200, 256, 256, 300),
			part(5, 256, 256, 512, 300),
			part(6, 512, 256, 600, 300),
		}},
		"past the edges of a world not wrapping": {flat, box(900, -50, 1100, 100), []sector.Part{
			part(3, 900, 0, 1000, 100),
		}},
		"wholly past an edge": {flat, box(1100, 10, 1200, 20), nil},
		"across the seam": {torus, box(900, 10, 1100, 20), []sector.Part{
			part(0, 0, 10, 76, 20),
			part(3, 900, 10, 1024, 20),
		}},
		"a period over": {torus, box(-100, 600, 50, 610), []sector.Part{
			part(0, 0, 88, 50, 98),
			part(3, 924, 88, 1024, 98),
		}},
		"wider than the world": {torus, box(-10, 0, 2000, 10), []sector.Part{
			part(0, 0, 0, 256, 10),
			part(1, 256, 0, 512, 10),
			part(2, 512, 0, 768, 10),
			part(3, 768, 0, 1024, 10),
		}},
		"both ends in one sector across the seam": {uneven, box(400, 0, 1300, 10), []sector.Part{
			part(0, 0, 0, 512, 10),
			part(1, 512, 0, 1000, 10),
		}},
	} {
		kept := []sector.Part{part(9, 1, 2, 3, 4)}
		got := c.g.Cover(c.box, kept)
		if !slices.Equal(got[1:], c.want) || got[0] != kept[0] {
			t.Errorf("%s: Cover(%v) = %v, want %v after what dst held", name, c.box, got[1:],
				c.want)
		}
	}
}
