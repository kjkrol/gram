package steering

// Pace is how fast the ground under an entity lets it go, a share of the speed it is steered
// at: 1 on open ground, a half where a step costs twice.
type Pace struct{ Share float32 }
