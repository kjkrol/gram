package main

import (
	"image/color"

	"github.com/kjkrol/gram/examples/island"
	"github.com/kjkrol/gram/plugins/atmosphere/weathering"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// snowyColors is how each kind snow may lie on looks under it; iceColor, water frozen.
var (
	snowyColors = map[string]color.RGBA{
		island.EarthCell:  {R: 232, G: 236, B: 235, A: 255},
		island.SandCell:   {R: 238, G: 236, B: 225, A: 255},
		island.RockCell:   {R: 205, G: 208, B: 212, A: 255},
		island.ForestCell: {R: 150, G: 185, B: 165, A: 255},
	}
	iceColor = color.RGBA{R: 175, G: 210, B: 230, A: 255}
)

func (s *mainStage) defineClimate() weathering.Config {
	kinds := s.board.CellKinds()
	snowy := map[string]string{}
	for name, col := range snowyColors {
		k, _ := kinds.Get(name)
		k.Name, k.Color = cell.Named(snowyCell(name)), col
		kinds.Create(k)
		snowy[name] = snowyCell(name)
	}
	kinds.Create(cell.Kind{Name: cell.Named(IceCell), Cost: 5, Allows: cell.Land | cell.Air, Color: iceColor}.Costing(cell.Air, 1))
	return weathering.Config{Snowy: snowy, Ice: IceCell, Sway: []string{island.ForestCell, snowyCell(island.ForestCell)}}
}
