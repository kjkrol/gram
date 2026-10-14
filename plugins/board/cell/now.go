package cell

import (
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/uid"
)

// Now is a cell at a step, as the rules the board hosts get it.
type Now struct {
	ID   uid.UID64 // the cell's entity
	Cell ID
	Kind Kind // effects on its ground included
}

// Who is the cell's entity.
func (n Now) Who() uid.UID64 { return n.ID }

// Placed marks the moment as a place: Around(0) acts on this cell, Around(n) on the rings round it.
func (Now) Placed() {}

var (
	_ host.About  = Now{}
	_ host.Placed = Now{}
)
