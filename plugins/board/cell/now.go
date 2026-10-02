package cell

import "github.com/kjkrol/uid"

// Now is a cell at a step, as the rules the board hosts get it: the cell's entity, which cell it
// is, and its kind now, effects on its ground included. The board runs its rules for every cell
// every step while one is hooked; they filter cells by the game's tags of places
// (rule.Self(trapdoor)) and by effects' markers (rule.Self(burning.Mark())).
type Now struct {
	ID      uid.UID64 // the cell's entity
	Cell    ID        // which cell
	Kind    Kind      // its kind now, effects on its ground included
	Trodden bool      // a unit stands on it now, the cell under its centre
}

// Stood reports whether a unit stands on the cell now: a plate pressed, for a rule's If.
func (n Now) Stood() bool { return n.Trodden }

// Who is the cell's entity: whose moment it is, for a rule.
func (n Now) Who() uid.UID64 { return n.ID }

// Placed: a rule's Here acts on this cell, its Around on the rings round it too.
func (Now) Placed() {}
