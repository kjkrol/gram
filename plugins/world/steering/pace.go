package steering

// Pace is how fast the ground under an entity lets it go: a share of the speed it is steered at,
// 1 on open ground, a half where a step costs twice. The plugin laying the ground — the board —
// writes it every step; the world's velocity pass reads it, before the rules of a Moving. An entity
// without one goes as it is steered.
type Pace struct{ Share float64 }
