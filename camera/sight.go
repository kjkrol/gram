package camera

// Sighted is a camera seeing a multiple of the world's sight, 1 as made.
type Sighted interface {
	Sight() float32
}

// SightOf is the multiple of the world's sight cam sees, 1 for a camera without one.
func SightOf(cam Camera) float32 {
	if s, ok := cam.(Sighted); ok {
		return s.Sight()
	}
	return 1
}

// Viewpoint is where cam looks from, the point its sight is measured on the ground from: its eye,
// else the point it observes; false for a camera knowing neither.
func Viewpoint(cam Camera) (x, y float32, ok bool) {
	if e, is := cam.(Eyed); is {
		if x, y, _, ok := e.Eye(); ok {
			return x, y, true
		}
	}
	if f, is := cam.(Focused); is {
		x, y, _ := f.Focus()
		return x, y, true
	}
	return 0, 0, false
}
