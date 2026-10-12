package entity

// Eye is where an entity looks from and how wide: Height above its bottom (Z.Altitude), zero its
// top; Angle the whole field of view across, radians.
type Eye struct{ Height, Angle float64 }

// Level is the height the eye looks from for an entity standing as z says: Height above its
// bottom, its top for none.
func (e Eye) Level(z Z) float64 {
	if e.Height != 0 {
		return z.Altitude + e.Height
	}
	return z.Top()
}
