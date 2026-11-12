package cell

import "github.com/kjkrol/aabbworld/geom"

import "github.com/kjkrol/ceremony/effect"

// On is the cell an entity standing on the board stands on for good (board.NewEntities).
type On struct{ Cell ID }

// Floor is a knob of an entity standing on a cell carrying the domains Allows over it at Height
// over its level, or sunk a negative Height under it, whatever its ground takes, at Pace (0: the
// ground's). With a Width it is a band that wide from the cell's middle out to each neighbour its
// Links name; without, the whole cell.
type Floor struct {
	Allows    Domain
	Height    float32
	Thickness float32
	// Load is the most whoever it carries may weigh, in kg (unit.Mover.Weight); 0 bears anyone.
	Load   float32
	Pace   float64
	Graded bool
	Links  Links
	Width  float32
	// Storey is which of the cell's storeys the floor is, the first 0: floors of one storey on a
	// cell are one floor, floors of two cross over each other.
	Storey uint8
	// Clear is the room over a sunk floor, up to its roof: whoever is taller stays out.
	Clear float32
	// Off is where the axis of a floor run as a band passes the cell, off the cell's middle, and
	// Lanes how many lanes it is marked with, as a Way's.
	Off   geom.Vec
	Lanes uint8
}

// Level is the level of its cell f makes: the ground's where it lies on it, else its storey's.
func (f Floor) Level() Level {
	if f.Height == 0 {
		return OnGround
	}
	return OnDeck + Level(f.Storey)
}

// Sunk reports whether f lies under the ground: a tunnel's floor, a cave's.
func (f Floor) Sunk() bool { return f.Height < 0 }

// Knob: an effect may turn it (effect.Knob).
func (Floor) Knob() {}

var _ effect.Knob = Floor{}

// Wall is a knob of an entity standing on a cell keeping the domains Keeps off it up to Height
// over its level, 0 at every height: a gate. Whoever fits through its Gap passes all the same.
type Wall struct {
	Keeps  Domain
	Height float32
	Gap    Gap
}

// Knob: an effect may turn it (effect.Knob).
func (Wall) Knob() {}

var _ effect.Knob = Wall{}

// Gap is an opening through a wall: a body no wider across its step than Width and no taller
// than Height passes, a zero Height open at every height; a zero Width is no opening.
type Gap struct {
	Width  float32
	Height float32
}

// Rail is a knob of an entity standing on a cell keeping the domains Keeps from crossing the edges
// of the cell's band, up to Height over the deck, 0 at every height: a bridge's railings, a fence.
type Rail struct {
	Keeps  Domain
	Height float32
}

// Knob: an effect may turn it (effect.Knob).
func (Rail) Knob() {}

var _ effect.Knob = Rail{}

// Built is what the entities standing on a cell build over and under it, gathered on the cell's
// entity: the floor of its first storey, the wall and the railing, and the floors of the storeys
// past the first.
type Built struct {
	Floor Floor
	Wall  Wall
	Rail  Rail
	More  [Levels - 2]Floor
}

// On is the floor making b's cell's level l, the zero Floor where none does.
func (b *Built) On(l Level) Floor {
	if l == OnGround {
		if b.Floor.Height == 0 {
			return b.Floor
		}
		return Floor{}
	}
	if f := b.Storey(uint8(l - OnDeck)); f.Height != 0 {
		return *f
	}
	return Floor{}
}

// Storey is b's floor of storey s.
func (b *Built) Storey(s uint8) *Floor {
	if s == 0 {
		return &b.Floor
	}
	return &b.More[s-1]
}

// Storeys is how many storeys a cell has.
const Storeys = Levels - 1

// Levelled reports whether a floor of b gives its cell a level beyond the ground's.
func (b *Built) Levelled() bool {
	return b.Floor.Height != 0 || b.More[0].Height != 0 || b.More[1].Height != 0
}

// Over is under as whoever crosses the cell meets it with what is built over it, on any level.
func (b Built) Over(under Kind) Kind {
	for s := range uint8(Storeys) {
		f := b.Storey(s)
		under.Allows |= f.Allows
		if f.Pace > 0 {
			for i := range under.Paces {
				if f.Allows&(1<<i) != 0 {
					under.Paces[i] = f.Pace
				}
			}
		}
		under.Graded = under.Graded || f.Allows != 0 && f.Graded
	}
	if b.Wall.Keeps != 0 {
		// the cell admits whom an opening lets through: a step refuses a body it does not fit
		if b.Wall.Gap.Width <= 0 {
			under.Allows &^= b.Wall.Keeps
		}
		under.Solid = true
	}
	return under
}
