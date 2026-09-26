package climate

import (
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/sky"
	"github.com/kjkrol/gram/plugins/world"
)

// Weathering is what a behaviour hosted by the weather hears every tick: the world's weather now
// and the season.
type Weathering struct {
	Weather world.Weather
	Season  sky.Season
}

// Every is a behaviour run every tick with the weather: where a game casts its effects as the
// weather says — snow lying while it snows in the frost, ice on the water, trees swaying in the
// wind — and takes them off again. Its Tick's Dt is the sky's time that tick: the tick at the
// day's pace, the jump the day was moved by, none while it stands. Register it with
// Plugin.RegisterBehavior.
func Every(react func(t plugin.Tick, w Weathering)) plugin.Behavior { return host.Every(react) }
