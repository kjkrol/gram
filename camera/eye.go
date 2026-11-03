package camera

// Eyed is a camera with an eye at a point of the world, a perspective's. Eye is where it stands,
// false while the camera has none.
type Eyed interface {
	Eye() (x, y, z float32, ok bool)
}

// Fronted is a camera some points lie behind the eye of, a perspective's. InFront reports
// whether the world point (x, y, z) lies before the eye, near enough to be drawn.
type Fronted interface {
	InFront(x, y, z float32) bool
}

// InFront reports whether the world point lies before cam's eye: every point does through a
// camera that is not Fronted.
func InFront(cam Camera, x, y, z float32) bool {
	if f, ok := cam.(Fronted); ok {
		return f.InFront(x, y, z)
	}
	return true
}
