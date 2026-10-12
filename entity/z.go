package entity

// Z is an entity's place in height: Altitude is its bottom, written by the board from the ground
// under it, Height its rise above that, Step how high it steps up onto a surface without a leap (0:
// a quarter of its Height). Only a world with heights carries it; see world.Config.Heights. Sight
// follows it, and collision meets two entities only where the heights they span overlap — one
// with no Height spans every height.
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
