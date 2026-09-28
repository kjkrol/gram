package main

import (
	"image/color"

	"github.com/kjkrol/gram/examples/island"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/topography"
)

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
	s.topography.Style("ice", topography.Style{Under: true, Shine: 0.3})
	relief := s.topography.Relief()
	return weathering.Config{Snowy: snowy, Ice: "ice", Sway: []string{"forest", "snowy forest"},
		High: func(c board.CellID) bool { return relief.Altitude(c) >= scale.Units(highSnow) }}
}
