// Package weather is the kinds of weather a climate goes through, each a [State]. [Default] is
// clear skies, fair ones, clouds, rain and storms; a game gives plugins/atmosphere/climate its
// own.
package weather

import (
	"time"

	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
)

// State is one kind of weather, its ranges thrown as it comes: Wind in world units a second,
// Clouds the share of the sky covered, Billow 0 torn shreds to 1 big heaps, Falls and Damp, the
// air's, 0 to 1, Warmth degrees against the season and the hour, Lasts at least and at most.
// Ceilings shares the low clouds among their bands under the peaks, covering them and over them
// (all zero: a third each); Next weighs which may follow (none: any other alike) and Often how
// often it comes in each calendar.Season (all zero: alike in every season).
type State struct {
	Name     string
	Wind     [2]float32
	Clouds   [2]float32
	Billow   [2]float32
	Falls    float32
	Damp     float32
	Warmth   float32
	Lasts    [2]time.Duration
	Ceilings [3]float32
	Next     map[string]float32
	Often    [4]float32
}

// The names of the Default weathers.
const (
	Clear  = "clear"
	Fair   = "fair"
	Cloudy = "cloudy"
	Rain   = "rain"
	Storm  = "storm"
)

// Default goes from a clear sky through fair and cloudy to rain, now and then a storm, and back.
var Default = []State{
	{
		Name:     Clear,
		Wind:     [2]float32{4, 14},
		Damp:     0.2,
		Warmth:   1,
		Lasts:    [2]time.Duration{45 * time.Second, 100 * time.Second},
		Ceilings: [3]float32{0.3, 0.15, 0.55},
		Next:     map[string]float32{Fair: 1},
	},
	{
		Name:     Fair,
		Wind:     [2]float32{4, 14},
		Clouds:   [2]float32{0.03, 0.25},
		Billow:   [2]float32{0.85, 1},
		Damp:     0.4,
		Warmth:   1,
		Lasts:    [2]time.Duration{45 * time.Second, 100 * time.Second},
		Ceilings: [3]float32{0.3, 0.15, 0.55},
		Next:     map[string]float32{Clear: 1, Cloudy: 1},
	},
	{
		Name:     Cloudy,
		Wind:     [2]float32{10, 25},
		Clouds:   [2]float32{0.4, 0.7},
		Billow:   [2]float32{0, 1},
		Damp:     0.6,
		Lasts:    [2]time.Duration{40 * time.Second, 90 * time.Second},
		Ceilings: [3]float32{0.2, 0.6, 0.2},
		Next:     map[string]float32{Fair: 1, Rain: 1},
	},
	{
		Name:     Rain,
		Wind:     [2]float32{15, 30},
		Clouds:   [2]float32{0.75, 0.9},
		Billow:   [2]float32{0.3, 1},
		Falls:    0.6,
		Damp:     0.9,
		Warmth:   -1,
		Lasts:    [2]time.Duration{30 * time.Second, 60 * time.Second},
		Ceilings: [3]float32{0.3, 0.6, 0.1},
		Next:     map[string]float32{Cloudy: 2, Storm: 0.5},
	},
	{
		Name:     Storm,
		Wind:     [2]float32{35, 55},
		Clouds:   [2]float32{0.9, 1},
		Billow:   [2]float32{0.7, 1},
		Falls:    1,
		Damp:     1,
		Warmth:   -3,
		Lasts:    [2]time.Duration{20 * time.Second, 40 * time.Second},
		Ceilings: [3]float32{0.2, 0.7, 0.1},
		Next:     map[string]float32{Rain: 1},
		Often: [4]float32{
			calendar.Spring: 0.6,
			calendar.Summer: 1.5,
			calendar.Autumn: 0.8,
			calendar.Winter: 0.3,
		},
	},
}

// Likely is how likely s comes in season: its Often for it, 1 when it has none for any.
func (s State) Likely(season calendar.Season) float32 {
	if s.Often == [4]float32{} {
		return 1
	}
	return s.Often[season%4]
}
