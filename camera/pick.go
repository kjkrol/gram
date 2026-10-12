package camera

// Picker is a camera that finds what is under a screen point itself. Pick is the first point the
// screen point (sx, sy) sees and its height; where it sees none, a point far along its line and
// false.
type Picker interface {
	Pick(sx, sy float32) (x, y, z float32, ok bool)
}
