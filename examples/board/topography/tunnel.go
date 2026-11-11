package main

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/underground"
	"github.com/kjkrol/gram/plugins/underground/site"
)

var tunnelClear = scale.Figure(2.2)

// a road's tunnel through 4 km of rock at most, climbing a metre in ten
var tunnelling = site.Rules{
	Longest: 40,
	Cover:   scale.Units(5),
	Grade:   0.1,
	Reshape: scale.Units(60),
}

// tunnel is a road's tunnel, two wagons wide and nearly twice as wide as it is high
type tunnel struct{}

var _ underground.Figure = tunnel{}

func (tunnel) Define(p *underground.Plugin) {
	p.Types().Define(TunnelKind, underground.Spec{
		Allows: cell.Land,
		Width:  scale.Figure(4),
		Clear:  tunnelClear,
		Pace:   1,
	})
}
