package camera

// Picker is a camera that finds the ground under a screen point itself — one drawing heights it
// knows. Pick is the first point of the ground the screen point (sx, sy) sees; where it sees none,
// the sky say, a point far along its line and false.
type Picker interface {
	Pick(sx, sy float32) (x, y float32, ok bool)
}
