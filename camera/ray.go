package camera

// Rayer is a camera with an eye that can say which way a screen point looks — a perspective's.
// Ray is the way from the eye through the screen point (sx, sy), a unit vector, and false for a
// camera drawing the world from infinitely far at the moment.
type Rayer interface {
	Ray(sx, sy float32) (dx, dy, dz float32, ok bool)
}
