package camera

// Rayer is a camera that can say which way a screen point looks. Ray is the way from the eye
// through the screen point (sx, sy), a unit vector, false for a camera without an eye at the
// moment.
type Rayer interface {
	Ray(sx, sy float32) (dx, dy, dz float32, ok bool)
}
