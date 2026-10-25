package entity

// Eye is where an entity looks from and how wide: Height above its bottom (Z.Altitude), zero its
// top; Angle the whole field of view across, radians; Vertical the field up and down, radians,
// zero every height; Tilt how far the eye is turned up, radians, down negative, which Vertical
// alone limits (a rider's camera writes it).
type Eye struct{ Height, Angle, Vertical, Tilt float32 }

// Within reports whether something spanning the elevations lo to hi from the eye, radians up,
// lies in its vertical field; everything does for a Vertical of zero.
func (e Eye) Within(lo, hi float32) bool {
	if e.Vertical <= 0 {
		return true
	}
	return hi >= e.Tilt-e.Vertical/2 && lo <= e.Tilt+e.Vertical/2
}

// Level is the height the eye looks from for an entity standing as z says: Height above its
// bottom, its top for none.
func (e Eye) Level(z Z) float32 {
	if e.Height != 0 {
		return z.Altitude + e.Height
	}
	return z.Top()
}
