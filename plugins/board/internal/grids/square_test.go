package grids

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
)

func TestSquareGrid_NonToroidal_EdgeExcludesNeighbors(t *testing.T) {
	g := NewSquare(4, 4, 10)
	corner := g.Neighbors(cellAtXY(g, 0, 0))
	if len(corner) != 3 {
		t.Errorf("corner cell has %d neighbors, want 3 (2 orthogonal + 1 diagonal, no wrap)", len(corner))
	}
}

func TestSquareGrid_Toroidal_EdgeWrapsNeighborsAndDistance(t *testing.T) {
	g := &Square{Width: 4, Height: 4, CellSize: 10, WrapX: true, WrapY: true}

	first, last := cellAtXY(g, 0, 0), cellAtXY(g, 3, 0)
	neighbors := g.Neighbors(first)
	if len(neighbors) != 8 {
		t.Fatalf("toroidal corner has %d neighbors, want 8", len(neighbors))
	}
	found := false
	for _, n := range neighbors {
		if n == last {
			found = true
		}
	}
	if !found {
		t.Error("expected column 0 and column Width-1 to be neighbors on a toroidal grid")
	}

	if d := g.Distance(first, last); d != 1 {
		t.Errorf("Distance(col 0, col Width-1) = %v, want 1 (wrap-around, not %d)", d, g.Width-1)
	}

}

func TestSquareGrid_Toroidal_CellAtWrapsNegativePositions(t *testing.T) {
	g := &Square{Width: 4, Height: 4, CellSize: 10, WrapX: true, WrapY: true}
	c, ok := g.CellAt(geom.NewVec(-5.0, -5.0))
	if !ok {
		t.Fatal("expected a negative position to wrap to a valid cell")
	}
	if c != cellAtXY(g, 3, 3) {
		t.Errorf("CellAt(-5,-5) = %v, want the wrapped last cell", c)
	}
}

func cellAtXY(g *Square, x, y uint32) cell.ID {
	c, _ := g.CellIndex(x, y)
	return c
}

func TestSquareGrid_CellIndex_NonToroidal(t *testing.T) {
	g := NewSquare(4, 4, 10)
	c, ok := g.CellIndex(2, 1)
	if !ok || c != cellAtXY(g, 2, 1) {
		t.Errorf("CellIndex(2,1) = (%v,%v), want (%v,true)", c, ok, cellAtXY(g, 2, 1))
	}
	if _, ok := g.CellIndex(4, 0); ok {
		t.Error("expected col==Width to be out of bounds on a non-toroidal grid")
	}
}

func TestSquareGrid_CellIndex_ToroidalWraps(t *testing.T) {
	g := &Square{Width: 4, Height: 4, CellSize: 10, WrapX: true, WrapY: true}
	c, ok := g.CellIndex(4, 0)
	origin, _ := g.CellIndex(0, 0)
	if !ok || c != origin {
		t.Errorf("CellIndex(4,0) = (%v,%v), want same cell as CellIndex(0,0)", c, ok)
	}
}

func TestSquareGrid_NeighborCost_OrthogonalVsDiagonal(t *testing.T) {
	g := NewSquare(4, 4, 10)
	orthogonal := g.NeighborCost(cellAtXY(g, 1, 1), cellAtXY(g, 1, 2))
	if orthogonal != 1 {
		t.Errorf("NeighborCost(orthogonal) = %v, want 1", orthogonal)
	}
	diagonal := g.NeighborCost(cellAtXY(g, 1, 1), cellAtXY(g, 2, 2))
	if want := math.Sqrt2; diagonal != want {
		t.Errorf("NeighborCost(diagonal) = %v, want %v", diagonal, want)
	}
}

func TestSquareGrid_Distance_OctileNeverOverestimatesDiagonalCost(t *testing.T) {
	g := NewSquare(4, 4, 10)
	d := g.Distance(cellAtXY(g, 0, 0), cellAtXY(g, 3, 3))
	if want := 3 * math.Sqrt2; d > want+1e-9 {
		t.Errorf("Distance(diagonal pair) = %v, want <= %v (true diagonal-move cost, must never overestimate)", d, want)
	}
}

func TestSquareGrid_DiagonalNeighbors_ReturnsFlankingCells(t *testing.T) {
	g := NewSquare(4, 4, 10)
	c1, c2, ok := g.DiagonalNeighbors(cellAtXY(g, 1, 1), cellAtXY(g, 2, 2))
	if !ok {
		t.Fatal("expected (1,1)->(2,2) to be recognized as a diagonal step")
	}
	want1, want2 := cellAtXY(g, 2, 1), cellAtXY(g, 1, 2)
	if (c1 != want1 || c2 != want2) && (c1 != want2 || c2 != want1) {
		t.Errorf("DiagonalNeighbors = (%v,%v), want (%v,%v) in either order", c1, c2, want1, want2)
	}
	if _, _, ok := g.DiagonalNeighbors(cellAtXY(g, 1, 1), cellAtXY(g, 1, 2)); ok {
		t.Error("expected an orthogonal pair to report ok=false")
	}
}

func TestSquareGrid_WrapsAlongOneAxisOnly(t *testing.T) {
	g := &Square{Width: 4, Height: 4, CellSize: 10, WrapX: true}
	corner := g.idAt(0, 0)

	got := map[cell.ID]bool{}
	for _, n := range g.Neighbors(corner) {
		got[n] = true
	}
	want := []cell.ID{g.idAt(1, 0), g.idAt(3, 0), g.idAt(0, 1), g.idAt(1, 1), g.idAt(3, 1)}
	if len(got) != len(want) {
		t.Fatalf("corner has %d neighbors, want %d: across the X seam but not the top", len(got), len(want))
	}
	for _, c := range want {
		if !got[c] {
			t.Errorf("neighbor %v missing", c)
		}
	}

	if d := g.Distance(g.idAt(0, 0), g.idAt(3, 0)); d != 1 {
		t.Errorf("distance across the X seam = %v, want 1", d)
	}
	if d := g.Distance(g.idAt(0, 0), g.idAt(0, 3)); d != 3 {
		t.Errorf("distance down a column = %v, want 3 — Y does not wrap", d)
	}
	if _, ok := g.CellAt(geom.NewVec(-5, 5)); !ok {
		t.Error("a position left of the world has no cell, want it wrapped")
	}
	if _, ok := g.CellAt(geom.NewVec(5, -5)); ok {
		t.Error("a position above the world has a cell, want none")
	}
	if _, ok := g.CellIndex(5, 0); !ok {
		t.Error("column 5 refused, want it wrapped")
	}
	if _, ok := g.CellIndex(0, 5); ok {
		t.Error("row 5 accepted, want it refused")
	}
}
