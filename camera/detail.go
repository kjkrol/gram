package camera

// Detail is how far a camera sees and how finely: Sight world units from where it looks (zero,
// without end), the last Fog of them fading into the fog, and Coarse the screen units a world
// unit spans below which what it shows is drawn coarsely (zero, never).
type Detail struct {
	Sight  float32
	Fog    float32
	Coarse float32
}

// Widened is d seeing factor times as far; a sight without end stays so.
func (d Detail) Widened(factor float32) Detail {
	d.Sight *= factor
	return d
}

// Detailed is a camera with a Detail of its own.
type Detailed interface {
	Detail() Detail
}

// DetailOf is cam's Detail, the zero one for a camera without.
func DetailOf(cam Camera) Detail {
	if d, ok := cam.(Detailed); ok {
		return d.Detail()
	}
	return Detail{}
}

// Viewpoint is where cam looks from, the point its Sight is measured on the ground from: its eye,
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
