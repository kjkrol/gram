package climate

import (
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/uid"
)

// Weathering is the moment a rule of the weather hears every step: the air over the world now
// and the season, about the atmosphere's own entity.
type Weathering struct {
	Weather air.Weather
	Season  calendar.Season
	Self    uid.UID64
}

// Who is the atmosphere's own entity.
func (w Weathering) Who() uid.UID64 { return w.Self }

var (
	_ host.About = Weathering{}
)
