package cell

// Domain is a bitset of ways of moving: what a cell admits and what an entity uses. Land, Water
// and Air are given; a game may add bits of its own.
type Domain uint8

const (
	Land Domain = 1 << iota
	Water
	Air
)
