package grids

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
)

func TestHexGrid_NonToroidal_EdgeExcludesNeighbors(t *testing.T) {
	g := NewHex(4, 4, 10)
	corner := g.Neighbors(g.At(0, 0))
	if len(corner) == 6 {
		t.Error("corner cell has 6 neighbors, want fewer (no wrap)")
	}
}

func TestHexGrid_Toroidal_AlwaysSixNeighbors(t *testing.T) {
	g := &Hex{Width: 4, Height: 4, Size: 10, WrapX: true, WrapY: true}
	for col := range g.Width {
		for row := range g.Height {
			n := g.Neighbors(g.At(col, row))
			if len(n) != 6 {
				t.Errorf("cell (%d,%d) has %d neighbors, want 6", col, row, len(n))
			}
		}
	}
}

func TestHexGrid_Toroidal_DistanceWrapsAtEdge(t *testing.T) {
	g := &Hex{Width: 4, Height: 4, Size: 10, WrapX: true, WrapY: true}
	for row := range g.Height {
		first, last := g.At(0, row), g.At(g.Width-1, row)
		if d := g.Distance(first, last); d != 1 {
			t.Errorf("row %d: the distance between its ends is %v, want 1 across the seam", row, d)
		}
	}
	for col := range g.Width {
		if d := g.Distance(g.At(col, 0), g.At(col, g.Height-1)); d != 1 {
			t.Errorf("column %d: the distance between its ends is %v, want 1 across the seam", col, d)
		}
	}
}

func TestHexGrid_Toroidal_NeighborsWrapToCanonicalRange(t *testing.T) {
	g := &Hex{Width: 4, Height: 4, Size: 10, WrapX: true, WrapY: true}
	for _, at := range [][2]uint32{{0, 0}, {3, 3}, {0, 3}, {3, 1}} {
		for _, n := range g.Neighbors(g.At(at[0], at[1])) {
			if col, row, ok := g.Coords(n); !ok {
				t.Errorf("a neighbour of %v is (%d, %d), off the board: want it wrapped", at, col, row)
			}
		}
	}
}

func TestHexGrid_At_NonToroidal(t *testing.T) {
	g := NewHex(4, 4, 10)
	// a row's cells begin half the row's number west of q 0
	if c := g.At(2, 1); c != packAxial(2, 1) {
		t.Errorf("At(2,1) = %v, want %v", c, packAxial(2, 1))
	}
	if c := g.At(2, 3); c != packAxial(1, 3) {
		t.Errorf("At(2,3) = %v, want %v", c, packAxial(1, 3))
	}
	if c := g.At(0, 2); c != packAxial(-1, 2) {
		t.Errorf("At(0,2) = %v, want %v", c, packAxial(-1, 2))
	}
	defer func() {
		if recover() == nil {
			t.Error("q==Width on a non-toroidal grid did not panic")
		}
	}()
	g.At(4, 0)
}

func TestHexGrid_At_ToroidalWraps(t *testing.T) {
	g := &Hex{Width: 4, Height: 4, Size: 10, WrapX: true, WrapY: true}
	if c, origin := g.At(4, 0), g.At(0, 0); c != origin {
		t.Errorf("At(4,0) = %v, want same cell as At(0,0) = %v", c, origin)
	}
}

func TestHexGrid_WrapsAlongOneAxisOnly(t *testing.T) {
	g := &Hex{Width: 4, Height: 4, Size: 10, WrapX: true}

	if got := len(g.Neighbors(g.At(0, 2))); got != 6 {
		t.Errorf("a cell on the left edge has %d neighbors, want all 6 through the X seam", got)
	}
	if got := len(g.Neighbors(g.At(2, 0))); got != 4 {
		t.Errorf("a cell on the top edge has %d neighbors, want 4 — Y does not wrap", got)
	}
	if d := g.Distance(g.At(0, 2), g.At(3, 2)); d != 1 {
		t.Errorf("distance across the X seam = %v, want 1", d)
	}
	if d := g.Distance(g.At(2, 0), g.At(2, 3)); d != 3 {
		t.Errorf("distance down a column = %v, want 3", d)
	}
	if _, _, ok := g.foldAxial(1, -1); ok {
		t.Error("a cell above the top row folds onto the grid, want not")
	}
	if _, _, ok := g.foldAxial(-1, 1); !ok {
		t.Error("a cell left of the first column does not fold onto the grid, want it wrapped")
	}
}

func TestHexGrid_DirectionsInOrder(t *testing.T) {
	g := NewHex(4, 4, 10)
	want := [6][2]int{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}
	if g.Directions() != len(want) {
		t.Fatalf("%d directions, want %d", g.Directions(), len(want))
	}
	for i, w := range want {
		if a, b := g.Offset(i); a != w[0] || b != w[1] {
			t.Errorf("direction %d is (%d, %d), want (%d, %d)", i, a, b, w[0], w[1])
		}
	}
}

func TestHexGrid_KeepsToARectangle(t *testing.T) {
	for _, size := range [][2]uint32{{6, 6}, {5, 7}, {1, 1}, {20, 12}} {
		g := NewHex(size[0], size[1], 10)
		var wide, tall float32
		left, top := float32(math.Inf(1)), float32(math.Inf(1))
		n := 0
		g.EachCell(func(c cell.ID) {
			o, ok := g.Ordinal(c)
			if !ok || o != n {
				t.Fatalf("%v: the cell %d of EachCell has the ordinal %d (%v)", size, n, o, ok)
			}
			n++
			col, row, ok := g.Coords(c)
			if !ok || g.At(col, row) != c || o != int(row*size[0]+col) {
				t.Fatalf("%v: the cell of ordinal %d is (%d, %d), %v", size, o, col, row, ok)
			}
			if at, ok := g.CellAt(g.CellCenter(c)); !ok || at != c {
				t.Fatalf("%v: the cell at the middle of (%d, %d) is another", size, col, row)
			}
			for _, p := range g.CellOutline(c, nil) {
				left, top = min(left, p.X), min(top, p.Y)
				wide, tall = max(wide, p.X), max(tall, p.Y)
			}
		})
		if n != g.CellCount() {
			t.Fatalf("%v: %d cells, want %d", size, n, g.CellCount())
		}
		// every other row half a cell east: half a cell wider than a row, however many rows
		half := float32(0.5)
		if size[1] == 1 {
			half = 0
		}
		w := float32(math.Sqrt(3)) * 10 * (float32(size[0]) + half)
		h := 10 * (1.5*float32(size[1]-1) + 2)
		if d := wide - left - w; d < -1e-2 || d > 1e-2 {
			t.Errorf("%v: the board is %g wide, want %g", size, wide-left, w)
		}
		if d := tall - top - h; d < -1e-2 || d > 1e-2 {
			t.Errorf("%v: the board is %g tall, want %g", size, tall-top, h)
		}
	}
}

func TestHexGrid_NeighboursAreAStepAway(t *testing.T) {
	g := NewHex(5, 6, 10)
	g.EachCell(func(c cell.ID) {
		for i := range g.Directions() {
			n, ok := g.Toward(c, i)
			if !ok {
				continue
			}
			if _, _, on := g.Coords(n); !on {
				t.Fatalf("a neighbour of %v lies off the board", c)
			}
			step, d := g.Step(i), g.CellCenter(n).Sub(g.CellCenter(c))
			if e := d.Sub(step); e.X*e.X+e.Y*e.Y > 1e-4 {
				t.Fatalf("the neighbour of %v the way %d lies %v off, want %v", c, i, d, step)
			}
		}
	})
	// off the board past a row's end, though the axial q is one a longer row has
	if _, ok := g.CellAt(g.CellCenter(packAxial(4, 2))); ok {
		t.Error("the axial (4, 2), the column 5 of the row 2, is on a board 5 wide")
	}
	if _, ok := g.Ordinal(packAxial(4, 2)); ok {
		t.Error("the axial (4, 2) has an ordinal on a board 5 wide")
	}
}

func TestHexGrid_AnOddHeightWrapsAlongXAlone(t *testing.T) {
	g := NewHex(4, 5, 10)
	g.SetWrap(true, false)
	defer func() {
		if recover() == nil {
			t.Error("a grid of five rows was set to wrap along y")
		}
	}()
	g.SetWrap(false, true)
}
