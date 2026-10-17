package cameras

// Cage is where the cameras may be: Free says whether nothing solid fills the point (x, y, z) —
// the ground, what stands on it, a deck, a cave's rock — and Span the lowest and highest heights
// the solid reaches. A line of sight stops where a point is not Free.
type Cage interface {
	Free(x, y, z float32) bool
	Span() (low, high float32)
}

// Grained is a Cage with things thinner than half a cell: Grain is how far a line of sight may
// step at (x, y) and how thin the thinnest thing there is, zero where half a cell will do.
type Grained interface {
	Grain(x, y float32) (step, thin float32)
}
