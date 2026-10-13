package driving

import (
	"github.com/kjkrol/ceremony/tag"
)

// States is the family of a unit's driving markers. The plugin gives it to every unit the world's
// roster makes; a unit without it gets it the first time it is needed.
type States struct{}

// Driving is on while a hand drives the unit; once no hand is on it and it stands, or something
// else steers it, the driving lets it go.
const Driving tag.Tag[States] = 0

const drivingName = "driving.driving"
