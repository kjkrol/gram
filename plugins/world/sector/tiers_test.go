package sector_test

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// five sectors across and three down, 100 a side
func tiered(wraps bool) *sector.Grid { return sector.NewGrid(500, 300, wraps, false, 100) }

func box(x0, y0, x1, y1 float32) geom.AABB {
	return geom.NewAABB(geom.NewVec(x0, y0), geom.NewVec(x1, y1))
}

func TestGrid_TiersClimbTillOneCellCoversTheWorld(t *testing.T) {
	g := tiered(false)
	if n := g.Tiers(); n != 4 {
		t.Fatalf("%d tiers over 5x3 sectors, want 0 to 3", n)
	}
	for k, want := range []int{15, 6, 2, 1} {
		if got := g.TierLen(k); got != want {
			t.Errorf("tier %d has %d cells, want %d", k, got, want)
		}
	}
	for _, c := range []struct {
		w, h uint32
		want int
	}{{100, 100, 1}, {200, 100, 2}, {400, 400, 3}, {500, 100, 4}} {
		if got := sector.NewGrid(c.w, c.h, false, false, 100).Tiers(); got != c.want {
			t.Errorf("%dx%d sectors of 100: %d tiers, want %d", c.w/100, c.h/100, got, c.want)
		}
	}
}

func TestGrid_TierBoxIsCutAtTheWorldsEdge(t *testing.T) {
	g := tiered(false)
	for _, c := range []struct {
		k, i int
		want geom.AABB
	}{
		{0, 7, g.Box(7)},
		{1, 0, box(0, 0, 200, 200)},
		{1, 2, box(400, 0, 500, 200)},
		{1, 5, box(400, 200, 500, 300)},
		{2, 1, box(400, 0, 500, 300)},
		{3, 0, box(0, 0, 500, 300)},
	} {
		if got := g.TierBox(c.k, c.i); got != c.want {
			t.Errorf("tier %d, cell %d: %v, want %v", c.k, c.i, got, c.want)
		}
	}
}

func TestGrid_UnderListsTheSectorsOfACell(t *testing.T) {
	g := tiered(false)
	under := func(k, i int) []int {
		var got []int
		g.Under(k, i, func(s int) { got = append(got, s) })
		return got
	}
	for _, c := range []struct {
		k, i int
		want []int
	}{
		{0, 7, []int{7}},
		{1, 0, []int{0, 1, 5, 6}},
		{1, 2, []int{4, 9}},
		{1, 4, []int{12, 13}},
		{1, 5, []int{14}},
		{2, 1, []int{4, 9, 14}},
		{3, 0, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}},
	} {
		if got := under(c.k, c.i); !slices.Equal(got, c.want) {
			t.Errorf("tier %d, cell %d holds sectors %v, want %v", c.k, c.i, got, c.want)
		}
	}
}

func TestGrid_TierOfHoldsAPointToTheWorld(t *testing.T) {
	g := tiered(false)
	for _, c := range []struct {
		k    int
		x, y float32
		want int
	}{
		{0, 450, 250, 14},
		{1, 199, 199, 0},
		{1, 200, 199, 1},
		{1, 450, 50, 2},
		{1, 450, 250, 5},
		{1, -50, 900, 3},
		{1, 900, -50, 2},
		{2, 399, 299, 0},
		{2, 400, 0, 1},
		{3, 499, 299, 0},
	} {
		if got := g.TierOf(c.k, c.x, c.y); got != c.want {
			t.Errorf("tier %d: (%v, %v) in cell %d, want %d", c.k, c.x, c.y, got, c.want)
		}
	}
}

func TestGrid_FitPutsAThingInTheLowestTierHoldingIt(t *testing.T) {
	g, wrapping := tiered(false), tiered(true)
	for _, c := range []struct {
		name       string
		g          *sector.Grid
		box        geom.AABB
		small      float32
		tier, cell int
	}{
		{"a small box over a seam", g, box(95, 10, 115, 20), 0, 0, 1},
		{"a box smaller than small", g, box(150, 10, 230, 20), 90, 0, 1},
		{"a box over small within a sector", g, box(110, 110, 190, 120), 0, 0, 6},
		{"a box over small across a sector's seam", g, box(70, 10, 130, 20), 0, 1, 0},
		{"a box 150 long within a cell of tier 1", g, box(20, 20, 170, 60), 0, 1, 0},
		{"a box 150 long across a seam of tier 1", g, box(120, 20, 270, 60), 0, 2, 0},
		{"a box across a seam of tier 2", g, box(350, 20, 450, 60), 0, 3, 0},
		{"a box as long as the world", g, box(0, 100, 500, 140), 0, 3, 0},
		{"a box past an open edge", g, box(480, 10, 520, 60), 0, 0, 4},
		{"a box across a wrapping edge", wrapping, box(480, 10, 520, 60), 0, 3, 0},
	} {
		if tier, cell := c.g.Fit(c.box, c.small); tier != c.tier || cell != c.cell {
			t.Errorf("%s: tier %d, cell %d; want tier %d, cell %d", c.name, tier, cell, c.tier,
				c.cell)
		}
	}
}
