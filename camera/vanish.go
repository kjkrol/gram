package camera

// Vanisher is a camera with vanishing points, a perspective's. Vanish is where the direction
// (dx, dy, dz) vanishes on the screen, false for one not ahead of the eye or a view without
// vanishing points at the moment.
type Vanisher interface {
	Vanish(dx, dy, dz float32) (sx, sy float32, ok bool)
}
