package climate

import (
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/uid"
)

// Weathering is what a rule hosted by the weather hears every step: the air over the world
// now and the season. It is about the atmosphere's own entity (the world's, for a climate run
// without one): a rule of it fires while the atmosphere plays its role, and an effect it applies
// is a state of the atmosphere.
type Weathering struct {
	Weather air.Weather
	Season  calendar.Season
	Self    uid.UID64
}

// Who is the atmosphere's own entity: whose moment it is, for a rule.
func (w Weathering) Who() uid.UID64 { return w.Self }
