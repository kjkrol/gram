package weather_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
)

func TestState_ComesAsLikelyAsItsSeasonSays(t *testing.T) {
	always := weather.State{Name: "clear"}
	if always.Likely(calendar.Winter) != 1 {
		t.Errorf("a weather without Often comes %v as likely in winter, want 1: alike every season", always.Likely(calendar.Winter))
	}
	storm := named("storm")
	if storm.Likely(calendar.Summer) <= storm.Likely(calendar.Winter) {
		t.Errorf("the default storm comes %v in summer and %v in winter, want it likelier in summer", storm.Likely(calendar.Summer), storm.Likely(calendar.Winter))
	}
}

// named is the default weather called name.
func named(name string) weather.State {
	for _, s := range weather.Default {
		if s.Name == name {
			return s
		}
	}
	panic("no default weather " + name)
}
