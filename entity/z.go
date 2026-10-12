package entity

// Z is an entity's place in height: Altitude its bottom, Height its rise above that, Step how
// high it steps up without a leap (0: a quarter of its Height). Only a world with heights carries
// it; one with no Height spans every height.
type Z struct{ Altitude, Height, Step float64 }

// Top is the entity's highest point.
func (z Z) Top() float64 { return z.Altitude + z.Height }

// Reach is how high the entity steps up: its Step, else a quarter of its Height.
func (z Z) Reach() float64 {
	if z.Step > 0 {
		return z.Step
	}
	return z.Height / 4
}
