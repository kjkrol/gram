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

// Links is which ways a Way runs on out of its cell: bit i the grid's i-th direction — on a square
// grid north, south, west, east, north-west, north-east, south-west, south-east; on a hex grid its
// six in turn. Link finds the bit for a neighbour, Toward the neighbour for a bit.
type Links uint8

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

// Crossing is what crosses a cell over its Way — a bridge over a river, a footbridge over a
// stream: a Way of its own, a band from the cell's middle out towards each neighbour its Links
// name. Its Kind lets whoever it admits over the cell too, at its cost, and leaves the way under it
// as it is: the water runs on under a bridge. The zero Crossing is none.
type Crossing struct{ Way }

// Over is under — the kind the ground and its Way make — as whoever crosses the cell meets it with
// the Crossing over it: it admits whoever either does, at the Crossing's cost for whoever it
// admits.
func (c Crossing) Over(under Kind) Kind {
	if !c.Runs() {
		return under
	}
	under.Allows |= c.Kind.Allows
	for i := range under.Costs {
		if d := Domain(1 << i); c.Kind.Allows&d != 0 {
			under.Costs[i] = c.Kind.CostFor(d)
		}
	}
	under.Graded = under.Graded || c.Kind.Graded
	return under
}
