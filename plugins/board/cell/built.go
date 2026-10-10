package cell

// On is the cell an entity standing on the board stands on for good (board.NewEntities).
type On struct{ Cell ID }

// Floor is a knob of an entity standing on a cell carrying the domains Allows over it at Height
// over its level, whatever its ground takes: a trapdoor over a pit, a bridge over a river.
type Floor struct {
	Allows Domain
	Height float64
}

// Wall is a knob of an entity standing on a cell keeping the domains Keeps off it up to Height
// over its level, 0 at every height: a gate.
type Wall struct {
	Keeps  Domain
	Height float64
}

// Built is what the entities standing on a cell build over it, gathered on the cell's entity.
type Built struct {
	Floor Floor
	Wall  Wall
}

// Over is under as whoever crosses the cell meets it with what is built over it.
func (b Built) Over(under Kind) Kind {
	under.Allows |= b.Floor.Allows
	if b.Wall.Keeps != 0 {
		under.Allows &^= b.Wall.Keeps
		under.Solid = true
	}
	return under
}
