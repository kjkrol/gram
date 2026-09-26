// Package weather is the kinds of weather a climate goes through: each State its wind, its clouds,
// what falls, how warm it is against the season, how long it lasts, which may follow it and in
// which seasons it comes. Default is clear skies, clouds, rain and storms; a game gives
// plugins/climate its own.
package weather

import (
	"time"

	"github.com/kjkrol/gram/plugins/sky"
)

// State is one kind of weather: the wind blowing between Wind[0] and Wind[1] world units a second,
// the Clouds covering the sky, what Falls — rain, snow when it is cold enough — 0 to 1 each, how
// much warmer or colder than the season and the hour it is (Warmth, degrees), how long it Lasts,
// at least and at most, which may follow it and how likely (Next, by name; none, any other alike),
// and how often it comes in each season, by sky.Season (Often; 0 never — all zero, alike in every
// season). How wet each season is is the climate's; Often is what is the weather's own, as
// thunderstorms in summer.
type State struct {
	Name   string
	Wind   [2]float32
	Clouds float32
	Falls  float32
	Warmth float32
	Lasts  [2]time.Duration
	Next   map[string]float32
	Often  [4]float32
}

// Default goes from clear skies to clouds, to rain — snow when it is cold enough — and now and then
// to a storm, most often in summer, and back.
var Default = []State{
	{Name: "clear", Wind: [2]float32{4, 14}, Clouds: 0.1, Warmth: 1, Lasts: [2]time.Duration{45 * time.Second, 100 * time.Second},
		Next: map[string]float32{"cloudy": 1}},
	{Name: "cloudy", Wind: [2]float32{10, 25}, Clouds: 0.6, Lasts: [2]time.Duration{40 * time.Second, 90 * time.Second},
		Next: map[string]float32{"clear": 1, "rain": 1}},
	{Name: "rain", Wind: [2]float32{15, 30}, Clouds: 0.85, Falls: 0.6, Warmth: -1, Lasts: [2]time.Duration{30 * time.Second, 60 * time.Second},
		Next: map[string]float32{"cloudy": 2, "storm": 0.5}},
	{Name: "storm", Wind: [2]float32{35, 55}, Clouds: 0.95, Falls: 1, Warmth: -3, Lasts: [2]time.Duration{20 * time.Second, 40 * time.Second},
		Next: map[string]float32{"rain": 1}, Often: [4]float32{sky.Spring: 0.6, sky.Summer: 1.5, sky.Autumn: 0.8, sky.Winter: 0.3}},
}

// Likely is how likely s comes in season: its Often for it, 1 when it has none for any.
func (s State) Likely(season sky.Season) float32 {
	if s.Often == [4]float32{} {
		return 1
	}
	return s.Often[season%4]
}
