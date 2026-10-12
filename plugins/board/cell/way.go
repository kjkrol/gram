package cell

// Way is what runs across a cell over its ground — a stream, a river, a road: a band Width wide
// from the cell's middle out towards each neighbour its Links name. Its Kind decides who may cross
// the cell and what it costs there, and how the band looks; the ground keeps the rest — whether it
// is solid, what it veils. Fade is how far it has faded out across the cell, 0 not at all to 1
// gone: a river running out into the sea. Mix is how far its look has turned into another's, 0 to 1,
// for a Dressing to show (plugins/topography: its kind's Style.MixWith): a river taking on the
// sea's colour towards its mouth. The zero Way is none.
type Way struct {
	Kind  Kind
	Width float32
	Links Links
	Fade  float32
	Mix   float32
}

// Links is which ways a Way runs on out of its cell: bit i the grid's i-th direction (grid.Grid,
// which numbers them). grid.Link finds the bit for a neighbour, Toward the neighbour for a bit.
type Links uint8

// Has reports whether the way runs out in direction i.
func (l Links) Has(i int) bool { return l&(1<<i) != 0 }

// Runs reports whether anything runs across the cell.
func (w Way) Runs() bool { return w.Width > 0 }

// Over is ground as whoever crosses the cell meets it: who may and what it costs from the Way's
// kind when one runs there, graded as it is, the rest from the ground.
func (w Way) Over(ground Kind) Kind {
	if !w.Runs() {
		return ground
	}
	ground.Allows, ground.Cost, ground.Costs, ground.Graded = w.Kind.Allows, w.Kind.Cost, w.Kind.Costs, w.Kind.Graded
	return ground
}
