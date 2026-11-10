package main

import (
	"image/color"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/underground"
	"github.com/kjkrol/gram/plugins/underground/site"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	tunnelWidth = 8
	tunnelClear = 3.5
)

var tunnelling = site.Rules{
	Longest: 10,
	Cover:   1,
	Grade:   0.12,
	Reshape: 3,
}

var portalColor = color.RGBA{R: 95, G: 95, B: 100, A: 255}

// tunnel is a road's tunnel, as wide as a deck and a wagon high
type tunnel struct{}

var _ underground.Figure = tunnel{}

func (tunnel) Define(p *underground.Plugin) {
	p.Types().Define(TunnelKind, underground.Spec{
		Allows: cell.Land,
		Width:  tunnelWidth,
		Clear:  tunnelClear,
		Pace:   1,
	})
}

func (tunnel) Look(a *world.Atlas, p *underground.Plugin) {
	a.Add(p.Types().Named(TunnelKind).Portal(), CellSize, render.Solid(portalColor)).Upright()
}
