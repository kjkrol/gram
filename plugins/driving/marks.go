package driving

import "github.com/kjkrol/gram/entity/tag"

// States is the family of a unit's driving markers. The plugin gives it to every unit the world's
// roster makes; a unit without it gets it the first time it is needed.
type States struct{}

// Driving is on while a hand drives the unit: a step without one brakes it, and once it stands —
// or something else steers it along a route of its own (Keeping.Ordered) — the driving lets it go.
const Driving tag.Tag[States] = 0

// drivingName is the name Driving is defined under, as the saves know it.
const drivingName = "driving.driving"
