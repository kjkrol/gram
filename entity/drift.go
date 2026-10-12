package entity

// Drift is the velocity an entity is carried at beyond the walk its steering gives it — falling,
// thrown, knocked away — in world units a second: X and Y the world moves its box by, with its
// Velocity, under one cap; Z its rise. Whoever carries it writes it (plugins/gravity).
type Drift struct{ X, Y, Z float64 }
