package unit_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
)

// a box up to a cell takes one; past it the fewest cells, two at least, holding its longer side
// with room to spare, an even number of them rounded up to an odd one on a hex board
func TestFootprint_TheFewestCellsHoldingABoxWithRoomToSpare(t *testing.T) {
	square := grid.DefaultGrids{}.Square(8, 8, 32)
	hex := grid.DefaultGrids{}.Hex(8, 8, 32)
	for _, tc := range []struct {
		side        float32
		square, hex int
	}{
		{0, 1, 1}, {32, 1, 1}, {33, 2, 3}, {59, 2, 3}, {60, 2, 3}, {63.9, 2, 3}, {64, 3, 3},
		{95, 3, 3}, {96, 4, 5},
	} {
		size := geom.NewVec(tc.side, 10)
		if got := unit.Footprint(square, size); got != tc.square {
			t.Errorf("a box %v long takes %d cells a side on a square board, want %d", tc.side,
				got, tc.square)
		}
		if got := unit.Footprint(hex, size); got != tc.hex {
			t.Errorf("a box %v long takes %d cells a side on a hex board, want %d", tc.side, got,
				tc.hex)
		}
	}
}

// a unit of an even footprint stands on the corner its middle cells share, half a cell right and
// down off the cell its footprint is laid from, which its place gives back; one of an odd footprint
// on its cell's middle
func TestFootprint_AnEvenFootprintStandsOnACornerAndItsCellIsReadBack(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, 32)
	c := g.At(3, 2)
	m := g.CellCenter(c)
	for _, tc := range []struct {
		size float32
		off  geom.Vec
	}{{20, geom.Vec{}}, {60, geom.NewVec(16, 16)}, {95, geom.Vec{}}, {120, geom.NewVec(16, 16)}} {
		size := geom.NewVec(tc.size, tc.size)
		place := unit.Place(g, size, c)
		if want := geom.NewVec(m.X+tc.off.X, m.Y+tc.off.Y); place != want {
			t.Errorf("a box %v wide laid from %v stands at %v, want %v", tc.size, c, place, want)
		}
		for _, jitter := range []geom.Vec{{}, geom.NewVec(0.5, -0.5), geom.NewVec(-3, 3)} {
			p := geom.NewVec(place.X+jitter.X, place.Y+jitter.Y)
			if back, _ := g.CellAt(unit.LaidFrom(g, size, p)); back != c {
				t.Errorf("a box %v wide standing at %v laid from %v, want %v", tc.size, p, back,
					c)
			}
		}
	}
}
