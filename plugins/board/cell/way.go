package cell

// Way is what runs across a cell over its ground, a band Width wide from the cell's middle out
// towards each neighbour its Links name; its Kind decides who may cross and what it costs. The
// zero Way is none.
type Way struct {
	Kind  Kind
	Width float32
	Links Links
	Fade  float32 // how far it has faded out across the cell, 0 to 1 gone
	Mix   float32 // how far its look has turned into another's, 0 to 1
}

// Links is which ways a Way runs on out of its cell: bit i the grid's i-th direction.
type Links uint8

// Has reports whether the way runs out in direction i.
func (l Links) Has(i int) bool { return l&(1<<i) != 0 }

// Runs reports whether anything runs across the cell.
func (w Way) Runs() bool { return w.Width > 0 }

// Over is ground as whoever crosses the cell meets it: who may, the cost and the grading from the
// Way's kind, the rest from the ground.
func (w Way) Over(ground Kind) Kind {
	if !w.Runs() {
		return ground
	}
	ground.Allows, ground.Cost, ground.Costs, ground.Graded = w.Kind.Allows, w.Kind.Cost, w.Kind.Costs, w.Kind.Graded
	return ground
}
