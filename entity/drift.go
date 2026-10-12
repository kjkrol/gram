package entity

// Drift is the velocity an entity is carried at beyond its walk (falling, thrown), in world units
// a second: X and Y along the ground, Z its rise. Gravity writes it.
type Drift struct{ X, Y, Z float64 }
