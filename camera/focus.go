package camera

// Focused is a camera that knows the world point it observes: the middle of what it shows, or the
// eye of what it rides in.
type Focused interface {
	Focus() (x, y, z float32)
}
