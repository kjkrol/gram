package cell

import "github.com/kjkrol/ceremony/effect"

// On is the cell an entity standing on the board stands on for good (board.NewEntities).
type On struct{ Cell ID }

// Floor is a knob of an entity standing on a cell carrying the domains Allows over it at Height
// over its level, whatever its ground takes, at Pace (0: the ground's). With a Width it is a band
// that wide from the cell's middle out to each neighbour its Links name; without, the whole cell.
type Floor struct {
	Allows    Domain
	Height    float32
	Thickness float32
	Pace      float64
	Graded    bool
	Links     Links
	Width     float32
}

// Knob: an effect may turn it (effect.Knob).
func (Floor) Knob() {}

var _ effect.Knob = Floor{}

// Wall is a knob of an entity standing on a cell keeping the domains Keeps off it up to Height
// over its level, 0 at every height: a gate.
type Wall struct {
	Keeps  Domain
	Height float32
}

// Knob: an effect may turn it (effect.Knob).
func (Wall) Knob() {}

var _ effect.Knob = Wall{}

// Rail is a knob of an entity standing on a cell keeping the domains Keeps from crossing the edges
// of the cell's band, up to Height over the deck, 0 at every height: a bridge's railings, a fence.
type Rail struct {
	Keeps  Domain
	Height float32
}

// Knob: an effect may turn it (effect.Knob).
func (Rail) Knob() {}

var _ effect.Knob = Rail{}

// Built is what the entities standing on a cell build over it, gathered on the cell's entity.
type Built struct {
	Floor Floor
	Wall  Wall
	Rail  Rail
}

// Over is under as whoever crosses the cell meets it with what is built over it.
func (b Built) Over(under Kind) Kind {
	under.Allows |= b.Floor.Allows
	if b.Floor.Pace > 0 {
		for i := range under.Paces {
			if b.Floor.Allows&(1<<i) != 0 {
				under.Paces[i] = b.Floor.Pace
			}
		}
	}
	under.Graded = under.Graded || b.Floor.Allows != 0 && b.Floor.Graded
	if b.Wall.Keeps != 0 {
		under.Allows &^= b.Wall.Keeps
		under.Solid = true
	}
	return under
}
