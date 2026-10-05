package entity

// Z is an entity's place in height: Altitude is its bottom, written by the board from the ground
// under it, Height its rise above that. Only a world with heights carries it; see
// world.Config.Heights. Sight follows it, and collision meets two entities only where the heights
// they span overlap — one with no Height spans every height.
type Z struct{ Altitude, Height float64 }

// Top is the entity's highest point.
func (z Z) Top() float64 { return z.Altitude + z.Height }
