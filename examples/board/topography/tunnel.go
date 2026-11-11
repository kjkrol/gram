package main

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/underground"
	"github.com/kjkrol/gram/plugins/underground/site"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var tunnelClear = scale.Figure(3)

// a road's tunnel through 4 km of rock at most, climbing a metre in ten
var tunnelling = site.Rules{
	Longest: 40,
	Cover:   scale.Units(5),
	Grade:   0.1,
	Reshape: scale.Units(60),
}

// tunnel is a road's tunnel, as wide as a bridge's deck and two wagons high
type tunnel struct{}

var _ underground.Figure = tunnel{}

func (tunnel) Define(p *underground.Plugin) {
	p.Types().Define(TunnelKind, underground.Spec{
		Allows: cell.Land,
		Width:  12,
		Clear:  tunnelClear,
		Pace:   1,
	})
}

func (tunnel) Look(a *world.Atlas, p *underground.Plugin) {
	a.Add(p.Types().Named(TunnelKind).Portal(), CellSize, render.Solid(pierColor)).Upright()
}
