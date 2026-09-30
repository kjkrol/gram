package climate

import (
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
)

// Weathering is what a trigger hosted by the weather hears every step: the air over the world
// now and the season.
type Weathering struct {
	Weather air.Weather
	Season  calendar.Season
}
