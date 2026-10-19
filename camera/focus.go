package camera

// Focused is a camera that knows the world point it observes: the middle of what it shows, or the
// eye of what it rides in.
type Focused interface {
	Focus() (x, y, z float32)
}

// Headed is a camera turned about the vertical: Ahead is the way along the ground that runs up its
// screen, a unit vector.
type Headed interface {
	Ahead() (x, y float32)
}
