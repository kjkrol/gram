package cell

// On is the cell an entity standing on the board stands on for good (board.NewEntities).
type On struct{ Cell ID }

// Floor is a knob of an entity standing on a cell carrying the domains Allows over it at Height
// over its level, whatever its ground takes, at Cost (0: the ground's) and spared the slope if
// Graded: a trapdoor over a pit, a ford waded slowly, a bridge over a river. Raised, it is a deck
// Thickness thick: whoever stands under it stays under it, whoever reaches it with a step stands
// on it. With a Width it is a band that wide from the cell's middle out to each neighbour its Links
// name, as a Way runs — a bridge's deck, the ground beside it the cell's own; one Link alone is a
// bridge's head, from the middle of its tile into the bridge; without a Width, the whole cell.
type Floor struct {
	Allows    Domain
	Height    float64
	Thickness float64
	Cost      float64
	Graded    bool
	Links     Links
	Width     float64
}

// Wall is a knob of an entity standing on a cell keeping the domains Keeps off it up to Height
// over its level, 0 at every height: a gate.
type Wall struct {
	Keeps  Domain
	Height float64
}

// Rail is a knob of an entity standing on a cell keeping the domains Keeps from crossing the edges
// of the cell's deck running as a band through it, up to Height over the deck, 0 at every height:
// a bridge's railings, a fence on a deck at the ground's level. Whoever leaps over its top
// crosses; with no band carrying anyone through it stands nowhere — a bridge's head has none.
type Rail struct {
	Keeps  Domain
	Height float64
}

// Built is what the entities standing on a cell build over it, gathered on the cell's entity.
type Built struct {
	Floor Floor
	Wall  Wall
	Rail  Rail
}

// Over is under as whoever crosses the cell meets it with what is built over it.
func (b Built) Over(under Kind) Kind {
	under.Allows |= b.Floor.Allows
	if b.Floor.Cost > 0 {
		for i := range under.Costs {
			if b.Floor.Allows&(1<<i) != 0 {
				under.Costs[i] = b.Floor.Cost
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
