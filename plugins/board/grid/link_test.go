package grid_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

func TestLink_FindsTheWayToEachNeighbourAndTowardFindsItBack(t *testing.T) {
	for _, g := range []grid.Grid{grid.DefaultGrids{}.Square(4, 4, 10), grid.DefaultGrids{}.Hex(4, 4, 10)} {
		c := g.At(1, 1)
		for _, n := range g.Neighbors(c) {
			l, ok := grid.Link(g, c, n)
			if !ok || l == 0 || l&(l-1) != 0 {
				t.Fatalf("link from %v to its neighbour %v: %b, %v; want one bit", c, n, l, ok)
			}
			i := 0
			for l>>i != 1 {
				i++
			}
			if back, ok := g.Toward(c, i); !ok || back != n {
				t.Errorf("toward %d from %v is %v, want %v", i, c, back, n)
			}
		}
		far := g.At(3, 3)
		if _, ok := grid.Link(g, c, far); ok {
			t.Errorf("a link from %v to %v, two cells off", c, far)
		}
	}
}

func TestDirections_AreTheGrids(t *testing.T) {
	for name, g := range map[string]grid.Grid{
		"square": grid.DefaultGrids{}.Square(5, 5, 10),
		"hex":    grid.DefaultGrids{}.Hex(5, 5, 10),
	} {
		c := g.At(2, 2)
		for i := range g.Directions() {
			n, ok := g.Toward(c, i)
			if !ok {
				t.Fatalf("%s: no neighbour %d of the middle cell", name, i)
			}
			if back, _ := g.Toward(n, g.Opposite(i)); back != c || g.Opposite(g.Opposite(i)) != i {
				t.Errorf("%s: %d's opposite %d does not lead back", name, i, g.Opposite(i))
			}
			from, to, s := g.CellCenter(c), g.CellCenter(n), g.Step(i)
			if math.Abs(to.X-from.X-s.X) > 1e-9 || math.Abs(to.Y-from.Y-s.Y) > 1e-9 {
				t.Errorf("%s: step %d is %v, the middles %v apart", name, i, s, to.Sub(from))
			}
			da, db := g.Offset(i)
			if d, ok := grid.Direction(g, da, db); !ok || d != i {
				t.Errorf("%s: the offset of %d names %d", name, i, d)
			}
			if d := grid.Nearest(g, s); d != i {
				t.Errorf("%s: the step of %d is nearest %d", name, i, d)
			}
			if w, _ := grid.Way(g, c, n); w != i {
				t.Errorf("%s: the way to neighbour %d is %d", name, i, w)
			}
			if l, _ := grid.Link(g, c, n); !cell.Links(l).Has(i) {
				t.Errorf("%s: the link to neighbour %d is %b", name, i, l)
			}
			a, b, ok := g.Flanks(i)
			if ok != g.Corner(i) {
				t.Fatalf("%s: %d is a corner %v, has flanks %v", name, i, g.Corner(i), ok)
			}
			if ok {
				c1, c2, _ := g.DiagonalNeighbors(c, n)
				na, _ := g.Toward(c, a)
				nb, _ := g.Toward(c, b)
				if !(na == c1 && nb == c2 || na == c2 && nb == c1) {
					t.Errorf("%s: %d's flanks %d, %d are not the cells beside its corner", name,
						i, a, b)
				}
			}
			var buf [8]geom.Vec
			corners := g.CellOutline(c, buf[:0])
			ra, rb := grid.Rim(g, c, i)
			e := grid.Exit(g, c, i)
			mid := geom.NewVec((corners[ra].X+corners[rb].X)/2, (corners[ra].Y+corners[rb].Y)/2)
			if math.Hypot(mid.X-e.X, mid.Y-e.Y) > 1e-9 || (ra == rb) != g.Corner(i) {
				t.Errorf("%s: %d leaves by corners %d, %d, not at its exit %v", name, i, ra, rb, e)
			}
		}
	}
}
