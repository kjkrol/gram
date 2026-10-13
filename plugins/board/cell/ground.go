package cell

import "github.com/kjkrol/ceremony/effect"

// Plot is which cell a cell's entity is.
type Plot struct{ Cell ID }

// Ground is the terrain kind of a cell, on the cell's entity: an effect altering it alters the
// board.
type Ground struct{ Kind Kind }

// Knob: an effect may turn it (effect.Knob).
func (Ground) Knob() {}

var _ effect.Knob = Ground{}
