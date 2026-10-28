package steering

// Pace is how fast the ground under an entity lets it go, a share of the speed it is steered
// at: 1 on open ground, a half where the board's mobility halves it, 0 where it refuses the
// step ahead.
type Pace struct{ Share float32 }
