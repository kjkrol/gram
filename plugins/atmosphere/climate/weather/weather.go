// Package weather is the kinds of weather a climate goes through: each State its wind, its clouds,
// what falls, how warm it is against the season, how long it lasts, which may follow it and in
// which seasons it comes. Default is clear skies, fair ones of a few big heaps, clouds, rain and
// storms; a game gives plugins/atmosphere/climate its own.
package weather

import (
	"time"

	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
)

// State is one kind of weather: the wind blowing between Wind[0] and Wind[1] world units a second,
// the Clouds covering between Clouds[0] and Clouds[1] of the sky, how heaped they are, between
// Billow[0] and Billow[1] (0 torn shreds, 1 big billowing heaps) — each thrown as the weather
// comes — what Falls — rain, snow when it is cold enough — 0 to 1 each, how
// much warmer or colder than the season and the hour it is (Warmth, degrees), how long it Lasts,
// at least and at most, which may follow it and how likely (Next, by name; none, any other alike),
// and how often it comes in each season, by calendar.Season (Often; 0 never — all zero, alike in every
// season). How wet each season is is the climate's; Often is what is the weather's own, as
// thunderstorms in summer.
type State struct {
	Name   string
	Wind   [2]float32
	Clouds [2]float32
	Billow [2]float32
	Falls  float32
	Warmth float32
	Lasts  [2]time.Duration
	Next   map[string]float32
	Often  [4]float32
}

// The names of the Default weathers: what climate.Config.Start and climate.Set say.
const (
	Clear  = "clear"
	Fair   = "fair"
	Cloudy = "cloudy"
	Rain   = "rain"
	Storm  = "storm"
)

// Default goes from a clear sky to a fair one, a few big heaps of cloud far apart, to clouds, to
// rain — snow when it is cold enough — and now and then to a storm, most often in summer, and back.
var Default = []State{
	{Name: Clear, Wind: [2]float32{4, 14}, Warmth: 1, Lasts: [2]time.Duration{45 * time.Second, 100 * time.Second},
		Next: map[string]float32{Fair: 1}},
	{Name: Fair, Wind: [2]float32{4, 14}, Clouds: [2]float32{0.03, 0.25}, Billow: [2]float32{0.85, 1}, Warmth: 1, Lasts: [2]time.Duration{45 * time.Second, 100 * time.Second},
		Next: map[string]float32{Clear: 1, Cloudy: 1}},
	{Name: Cloudy, Wind: [2]float32{10, 25}, Clouds: [2]float32{0.4, 0.7}, Billow: [2]float32{0, 1}, Lasts: [2]time.Duration{40 * time.Second, 90 * time.Second},
		Next: map[string]float32{Fair: 1, Rain: 1}},
	{Name: Rain, Wind: [2]float32{15, 30}, Clouds: [2]float32{0.75, 0.9}, Billow: [2]float32{0.3, 1}, Falls: 0.6, Warmth: -1, Lasts: [2]time.Duration{30 * time.Second, 60 * time.Second},
		Next: map[string]float32{Cloudy: 2, Storm: 0.5}},
	{Name: Storm, Wind: [2]float32{35, 55}, Clouds: [2]float32{0.9, 1}, Billow: [2]float32{0.7, 1}, Falls: 1, Warmth: -3, Lasts: [2]time.Duration{20 * time.Second, 40 * time.Second},
		Next: map[string]float32{Rain: 1}, Often: [4]float32{calendar.Spring: 0.6, calendar.Summer: 1.5, calendar.Autumn: 0.8, calendar.Winter: 0.3}},
}

// Likely is how likely s comes in season: its Often for it, 1 when it has none for any.
func (s State) Likely(season calendar.Season) float32 {
	if s.Often == [4]float32{} {
		return 1
	}
	return s.Often[season%4]
}
