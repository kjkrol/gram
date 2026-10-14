package main

import (
	"time"

	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// weather.Default written out to play with: wind in world units a second, clouds, billow and
// what falls 0 to 1, each thrown between its two numbers
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

const highSnow = 480 // metres: the ground snow lies on first

var snowable = []string{EarthCell, SandCell, RockCell, ForestCell}

func (a *arena) defineWinterCells() {
	kinds := a.board.CellKinds()
	for _, name := range snowable {
		k, _ := kinds.Get(name)
		kinds.Define(snowyCell(name), k)
	}
	kinds.Define(IceCell, cell.Kind{Cost: 5, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1))
}

func (a *arena) defineClimate() weathering.Config {
	snowy := map[string]string{}
	for _, name := range snowable {
		snowy[name] = snowyCell(name)
	}
	ground := a.topography.Relief()
	return weathering.Config{
		Snowy: snowy,
		Ice:   IceCell,
		Sway:  []string{ForestCell, snowyCell(ForestCell)},
		High:  func(c cell.ID) bool { return ground.Altitude(c) >= scale.Units(highSnow) },
	}
}
