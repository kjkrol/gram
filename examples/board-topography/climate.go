package main

import (
	"image/color"
	"time"

	"github.com/kjkrol/gram/examples/island"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/topography/painter"
)

// weathers is the weather the island goes through, weather.Default's written out to play with: a
// clear sky, a fair one of a few big heaps, clouds, rain, storms. Each has its wind (world units a
// second), cloud cover (0 to 1) and how heaped the clouds are (0 torn shreds, 1 big heaps) thrown
// between the two numbers as it comes, what falls (0 to 1), how much warmer than the hour and the
// season it is, how long it lasts, what may follow it and how likely, and in which seasons it
// comes (none: alike in every season).
var weathers = []weather.State{
	{Name: "clear", Wind: [2]float32{4, 14}, Warmth: 1,
		Lasts: [2]time.Duration{45 * time.Second, 100 * time.Second}, Next: map[string]float32{"fair": 1}},
	{Name: "fair", Wind: [2]float32{4, 14}, Clouds: [2]float32{0.03, 0.25}, Billow: [2]float32{0.85, 1}, Warmth: 1,
		Lasts: [2]time.Duration{45 * time.Second, 100 * time.Second}, Next: map[string]float32{"clear": 1, "cloudy": 1}},
	{Name: "cloudy", Wind: [2]float32{10, 25}, Clouds: [2]float32{0.4, 0.7}, Billow: [2]float32{0, 1},
		Lasts: [2]time.Duration{40 * time.Second, 90 * time.Second}, Next: map[string]float32{"fair": 1, "rain": 1}},
	{Name: "rain", Wind: [2]float32{15, 30}, Clouds: [2]float32{0.75, 0.9}, Billow: [2]float32{0.3, 1}, Falls: 0.6, Warmth: -1,
		Lasts: [2]time.Duration{30 * time.Second, 60 * time.Second}, Next: map[string]float32{"cloudy": 2, "storm": 0.5}},
	{Name: "storm", Wind: [2]float32{35, 55}, Clouds: [2]float32{0.9, 1}, Billow: [2]float32{0.7, 1}, Falls: 1, Warmth: -3,
		Lasts: [2]time.Duration{20 * time.Second, 40 * time.Second}, Next: map[string]float32{"rain": 1},
		Often: [4]float32{calendar.Spring: 0.6, calendar.Summer: 1.5, calendar.Autumn: 0.8, calendar.Winter: 0.3}},
}

// highSnow is how high the ground is, in metres, that snow lies on first: 60 of the island's
// heights.
const highSnow = 60 * island.Metres

// snowyColors is how each kind snow may lie on looks under it; iceColor, water frozen.
var (
	snowyColors = map[string]color.RGBA{
		"earth":  {R: 232, G: 236, B: 235, A: 255},
		"sand":   {R: 238, G: 236, B: 225, A: 255},
		"rock":   {R: 205, G: 208, B: 212, A: 255},
		"forest": {R: 150, G: 185, B: 165, A: 255},
	}
	iceColor = color.RGBA{R: 175, G: 210, B: 230, A: 255}
)

// defineClimate adds the snowy kinds and ice to the board's dictionary — the same ground to cross
// and stand on, another look, in the snow's and the ice's colours — and says how the weather lies
// on the island; call it after the island's own kinds.
func (s *mainStage) defineClimate() weathering.Config {
	kinds := s.board.CellKindDict()
	snowy := map[string]string{}
	for name, col := range snowyColors {
		k, _ := kinds.Get(name)
		k.Name, k.Color = board.Named("snowy "+name), col
		kinds.Create(k)
		s.topography.Style("snowy "+name, s.topography.StyleOf(name))
		snowy[name] = "snowy " + name
	}
	kinds.Create(board.CellKind{Name: board.Named("ice"), Cost: 5, Allows: board.Land | board.Air, Color: iceColor}.Costing(board.Air, 1))
	s.topography.Style("ice", painter.Style{Under: true, Shine: 0.3})
	ground := s.topography.Relief()
	return weathering.Config{Snowy: snowy, Ice: "ice", Sway: []string{"forest", "snowy forest"},
		High: func(c board.CellID) bool { return ground.Altitude(c) >= scale.Units(highSnow) }}
}
