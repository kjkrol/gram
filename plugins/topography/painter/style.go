package painter

// Style is how a kind of the board looks in relief beyond its sprite, set by the kind's name.
// Shine (0 to 1) is how much of the sun it throws back, Flow how fast water runs down its slope
// (world units a second at a fall of 1 in 1, 0 still), Spread (0 to a half) how softly it runs
// into a spreading neighbour, Under whether it lies under the kinds round it, MixWith the kind
// a way of it takes the look of towards its mouth.
type Style struct {
	Shine   float32
	Flow    float32
	Spread  float32
	Under   bool
	MixWith string
}
