package camera

// Scroller is a camera the cursor at a window's edge scrolls on its own terms: in a corner it
// may turn where at a side it scrolls. One that is none is panned.
type Scroller interface {
	Scroll(dx, dy float32, corner bool)
}
