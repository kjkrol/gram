package light

// Lights are a scene's lights for a frame: the distant light, the lamps, how many lamps nearest a
// camera cast shadows, whether the ground's shade is baked coarse, how a live shadow is cast by
// its distance from the eye (nil: as declared), the blocks whose shadows the ground bakes, the
// ground they are baked over (one of under two corners a side: none), and the height of the
// ground under a point (nil: level at 0, where the lamps cast shadows too).
type Lights struct {
	Sun         Distant
	Lamps       []Lamp
	LampShadows int
	Coarse      bool
	Policy      ShadowPolicy
	Blocks      []Block
	Ground      Lattice
	GroundAt    func(x, y float32) float32
}

// Block is a solid whose shadow the ground bakes: a rectangle round (X, Y) running (UX, UY),
// Along from its middle that way and Wide across, standing from Bottom up to Top.
type Block struct{ X, Y, UX, UY, Along, Wide, Bottom, Top float32 }

// Lattice is the ground the shade is baked over: Cols by Rows corners Cell world units apart from
// the origin, the lowest Low high and the highest Low+Span; Version counts the changes to the
// heights. Level ground is a Lattice of no Span.
type Lattice struct {
	Cols, Rows int
	Cell       float32
	Low, Span  float32
	Version    uint64
}
