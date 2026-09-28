package topography

// Style is how a kind of the board looks beyond its sprite, set by the kind's name (Plugin.Style).
// Shine is how much of the sun its surface throws back, 0 to 1: water, ice, wet rock glint where
// the sun and the eye meet over its ripples. Flow is how fast water on it runs down its slope, world
// units a second where it falls 1 in 1, by the square root of the slope; 0 is still. Spread is how
// softly it runs into a neighbour of another kind that spreads too, 0 to a half, the two meeting
// along a line their cells draw; 0 keeps its cells square. Under has it lie under the kinds round
// it — water: a tile that spreads next to it is drawn as it, glinting and all, its own kind laid
// over along the line, so a coast runs round. A way's kind is styled the same way, and MixWith
// names the kind whose look a way of it turns into as far as its board.Way.Mix says: a river
// taking on the sea's colour towards its mouth.
type Style struct {
	Shine   float32
	Flow    float32
	Spread  float32
	Under   bool
	MixWith string
}
