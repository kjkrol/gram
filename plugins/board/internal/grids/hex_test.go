package grids

import (
	"testing"
)

func TestHexGrid_NonToroidal_EdgeExcludesNeighbors(t *testing.T) {
	g := NewHex(4, 4, 10)
	corner := g.Neighbors(packAxial(0, 0))
	if len(corner) == 6 {
		t.Error("corner cell has 6 neighbors, want fewer (no wrap)")
	}
}

func TestHexGrid_Toroidal_AlwaysSixNeighbors(t *testing.T) {
	g := &Hex{Width: 4, Height: 4, Size: 10, WrapX: true, WrapY: true}
	for q := int32(0); q < int32(g.Width); q++ {
		for r := int32(0); r < int32(g.Height); r++ {
			n := g.Neighbors(packAxial(q, r))
			if len(n) != 6 {
				t.Errorf("cell (%d,%d) has %d neighbors, want 6", q, r, len(n))
			}
		}
	}
}

func TestHexGrid_Toroidal_DistanceWrapsAtEdge(t *testing.T) {
	g := &Hex{Width: 4, Height: 4, Size: 10, WrapX: true, WrapY: true}
	first := packAxial(0, 0)
	last := packAxial(int32(g.Width-1), 0)

	if d := g.Distance(first, last); d != 1 {
		t.Errorf("Distance(q=0, q=Width-1) = %v, want 1 (wrap-around, not %d)", d, g.Width-1)
	}
}

func TestHexGrid_Toroidal_NeighborsWrapToCanonicalRange(t *testing.T) {
	g := &Hex{Width: 4, Height: 4, Size: 10, WrapX: true, WrapY: true}
	for _, n := range g.Neighbors(packAxial(0, 0)) {
		q, r := unpackAxial(n)
		if q < 0 || r < 0 || uint32(q) >= g.Width || uint32(r) >= g.Height {
			t.Errorf("neighbor (%d,%d) is outside the canonical range, want it wrapped", q, r)
		}
	}
}

func TestHexGrid_CellIndex_NonToroidal(t *testing.T) {
	g := NewHex(4, 4, 10)
	c, ok := g.CellIndex(2, 1)
	if !ok || c != packAxial(2, 1) {
		t.Errorf("CellIndex(2,1) = (%v,%v), want (%v,true)", c, ok, packAxial(2, 1))
	}
	if _, ok := g.CellIndex(4, 0); ok {
		t.Error("expected q==Width to be out of bounds on a non-toroidal grid")
	}
}

func TestHexGrid_CellIndex_ToroidalWraps(t *testing.T) {
	g := &Hex{Width: 4, Height: 4, Size: 10, WrapX: true, WrapY: true}
	c, ok := g.CellIndex(4, 0)
	origin, _ := g.CellIndex(0, 0)
	if !ok || c != origin {
		t.Errorf("CellIndex(4,0) = (%v,%v), want same cell as CellIndex(0,0)", c, ok)
	}
}

func TestHexGrid_WrapsAlongOneAxisOnly(t *testing.T) {
	g := &Hex{Width: 4, Height: 4, Size: 10, WrapX: true}

	if got := len(g.Neighbors(packAxial(0, 2))); got != 6 {
		t.Errorf("a cell on the left edge has %d neighbors, want all 6 through the X seam", got)
	}
	if got := len(g.Neighbors(packAxial(2, 0))); got != 4 {
		t.Errorf("a cell on the top edge has %d neighbors, want 4 — Y does not wrap", got)
	}
	if d := g.Distance(packAxial(0, 2), packAxial(3, 2)); d != 1 {
		t.Errorf("distance across the X seam = %v, want 1", d)
	}
	if d := g.Distance(packAxial(2, 0), packAxial(2, 3)); d != 3 {
		t.Errorf("distance down a column = %v, want 3", d)
	}
	if _, _, ok := g.foldAxial(1, -1); ok {
		t.Error("a cell above the top row folds onto the grid, want not")
	}
	if _, _, ok := g.foldAxial(-1, 1); !ok {
		t.Error("a cell left of the first column does not fold onto the grid, want it wrapped")
	}
}
