package climate

import (
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/uid"
)

// Weathering is what a rule hosted by the weather hears every step: the air over the world
// now and the season. It is about the world's own entity, the clock's: an effect a rule applies
// on it is a state of the whole game.
type Weathering struct {
	Weather air.Weather
	Season  calendar.Season
	World   uid.UID64
}

// Who is the world's own entity: whose moment it is, for a rule.
func (w Weathering) Who() uid.UID64 { return w.World }
