package weather_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/climate/weather"
	"github.com/kjkrol/gram/plugins/sky"
)

func TestState_ComesAsLikelyAsItsSeasonSays(t *testing.T) {
	always := weather.State{Name: "clear"}
	if always.Likely(sky.Winter) != 1 {
		t.Errorf("a weather without Often comes %v as likely in winter, want 1: alike every season", always.Likely(sky.Winter))
	}
	storm := weather.Default[3]
	if storm.Likely(sky.Summer) <= storm.Likely(sky.Winter) {
		t.Errorf("the default storm comes %v in summer and %v in winter, want it likelier in summer", storm.Likely(sky.Summer), storm.Likely(sky.Winter))
	}
}
