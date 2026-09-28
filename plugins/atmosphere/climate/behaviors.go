package climate

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/world"
)

// Weathering is what a behaviour hosted by the weather hears every step: the world's weather now
// and the season.
type Weathering struct {
	Weather world.Weather
	Season  calendar.Season
}

// Every is a behaviour run every step of the simulation with the weather: where a game reads the
// weather as it goes — its Tick's Dt the step. What lies on the ground as the weather says — snow,
// ice, trees swaying — is plugins/atmosphere/weathering's; register Every with
// atmosphere.Plugin.RegisterBehavior.
func Every(react func(t plugin.Tick, w Weathering)) plugin.Behavior { return host.Every(react) }
