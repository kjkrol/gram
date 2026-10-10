package main

import (
	"time"

	"github.com/kjkrol/gram/examples/island"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/topography/painter"
)

// weathers is the weather the island goes through, weather.Default's written out to play with: a
// clear sky, a fair one of a few big heaps, clouds, rain, storms. Each has its wind (world units a
// second), cloud cover (0 to 1) and how heaped the clouds are (0 torn shreds, 1 big heaps) thrown
// between the two numbers as it comes, what falls (0 to 1), how much warmer than the hour and the
// season it is, how long it lasts, what may follow it and how likely, and in which seasons it
// comes (none: alike in every season).
var weathers = []weather.State{
	{
		Name:   weather.Clear,
		Wind:   [2]float32{4, 14},
		Warmth: 1,
		Lasts:  [2]time.Duration{45 * time.Second, 100 * time.Second},
		Next:   map[string]float32{weather.Fair: 1},
	},
	{
		Name:   weather.Fair,
		Wind:   [2]float32{4, 14},
		Clouds: [2]float32{0.03, 0.25},
		Billow: [2]float32{0.85, 1},
		Warmth: 1,
		Lasts:  [2]time.Duration{45 * time.Second, 100 * time.Second},
		Next:   map[string]float32{weather.Clear: 1, weather.Cloudy: 1},
	},
	{
		Name:   weather.Cloudy,
		Wind:   [2]float32{10, 25},
		Clouds: [2]float32{0.4, 0.7},
		Billow: [2]float32{0, 1},
		Lasts:  [2]time.Duration{40 * time.Second, 90 * time.Second},
		Next:   map[string]float32{weather.Fair: 1, weather.Rain: 1},
	},
	{
		Name:   weather.Rain,
		Wind:   [2]float32{15, 30},
		Clouds: [2]float32{0.75, 0.9},
		Billow: [2]float32{0.3, 1},
		Falls:  0.6,
		Warmth: -1,
		Lasts:  [2]time.Duration{30 * time.Second, 60 * time.Second},
		Next:   map[string]float32{weather.Cloudy: 2, weather.Storm: 0.5},
	},
	{
		Name:   weather.Storm,
		Wind:   [2]float32{35, 55},
		Clouds: [2]float32{0.9, 1},
		Billow: [2]float32{0.7, 1},
		Falls:  1,
		Warmth: -3,
		Lasts:  [2]time.Duration{20 * time.Second, 40 * time.Second},
		Next:   map[string]float32{weather.Rain: 1},
		Often: [4]float32{
			calendar.Spring: 0.6,
			calendar.Summer: 1.5,
			calendar.Autumn: 0.8,
			calendar.Winter: 0.3,
		},
	},
}

// highSnow is how high the ground is, in metres, that snow lies on first: 60 of the island's
// heights.
const highSnow = 60 * island.Metres

// snowable are the kinds snow may lie on, each with a snowy twin.
var snowable = []string{island.EarthCell, island.SandCell, island.RockCell, island.ForestCell}

func (s *arena) defineWinterCells() {
	kinds := s.board.CellKinds()
	for _, name := range snowable {
		k, _ := kinds.Get(name)
		kinds.Define(snowyCell(name), k)
		s.topography.Style(snowyCell(name), s.topography.StyleOf(name))
	}
	kinds.Define(IceCell, cell.Kind{Cost: 5, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1))
	s.topography.Style(IceCell, painter.Style{Under: true, Shine: 0.3})
}

func (s *arena) defineClimate() weathering.Config {
	snowy := map[string]string{}
	for _, name := range snowable {
		snowy[name] = snowyCell(name)
	}
	ground := s.topography.Relief()
	return weathering.Config{
		Snowy: snowy,
		Ice:   IceCell,
		Sway:  []string{island.ForestCell, snowyCell(island.ForestCell)},
		High:  func(c cell.ID) bool { return ground.Altitude(c) >= scale.Units(highSnow) },
	}
}
