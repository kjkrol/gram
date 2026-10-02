package camera

// Vanisher is a camera through a projection with vanishing points — a perspective's. Vanish is
// where the direction (dx, dy, dz) vanishes on the screen — where everything far along it, the
// sun say, is drawn — and false for a direction not ahead of the eye, or a view without vanishing
// points at the moment.
type Vanisher interface {
	Vanish(dx, dy, dz float32) (sx, sy float32, ok bool)
}
