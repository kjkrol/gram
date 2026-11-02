package site

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// Ground is what a road is planned over: a board and its heights, as the planner reads them.
type Ground interface {
	Grid() grid.Grid
	// Heights are the ground's heights at c's outline, in CellOutline's order; level ground
	// without a relief.
	Heights(c cell.ID, dst []float32) []float32
	// Takes reports whether c's ground may take a road.
	Takes(c cell.ID) bool
	// Wet reports whether water runs over c.
	Wet(c cell.ID) bool
	// Road is the links of the road running through c; none for none.
	Road(c cell.ID) cell.Links
	// Decked is the links of the piece of a bridge's deck lying over c, a road may run on: the
	// way the deck runs, a head's one link toward the span; false without a deck.
	Decked(c cell.ID) (cell.Links, bool)
	// Ford reports whether c is a ford, a road may run through, whatever Wet says of it.
	Ford(c cell.ID) bool
	// Fixed reports whether c's corners may never move: water's bed, what stands on the cell, a
	// bridge's head.
	Fixed(c cell.ID) bool
	// Held reports whether something stands on c that the levelling would move, a unit: the
	// plan waits for the cell to be free.
	Held(c cell.ID) bool
	// Price is how dear a road over c is for its ground, 1 the plain price of a cell and the
	// least, a lower one counted as 1; Route weighs its steps by it.
	Price(c cell.ID) float32
}
