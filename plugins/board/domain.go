package board

// Domain is a bitset of ways of moving: what a cell admits and what an entity uses. Land, Water
// and Air are given; a game may add bits of its own.
type Domain uint8

const (
	Land Domain = 1 << iota
	Water
	Air
)

// Mover says which domains an entity moves in and, in a world with heights, how far above the ground
// it keeps (Lift: a hawk hundreds of metres, a walker 0), the least it keeps over the ground
// (Clearance) and the highest over sea level it climbs to (Ceiling; 0 none). One that flies (Air),
// flown by hand (steering.Driven.Flown), holds its height over sea level instead, climbing and
// diving as it is steered, its Lift following; an entity on the board without it moves on Land.
type Mover struct {
	Domain    Domain
	Lift      float64
	Clearance float64
	Ceiling   float64
}

// DomainAt is the domain of the i-th entity of a chunk whose Mover column may be absent.
func DomainAt(movers []Mover, i int) Domain {
	if movers == nil {
		return Land
	}
	return movers[i].Domain
}
