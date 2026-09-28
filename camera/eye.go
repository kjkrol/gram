package camera

// Eyed is a camera with an eye at a point of the world — a perspective's: Eye is where it stands,
// false while the camera has none, drawing the world from infinitely far.
type Eyed interface {
	Eye() (x, y, z float32, ok bool)
}
