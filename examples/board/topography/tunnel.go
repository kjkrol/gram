package main

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/underground"
	"github.com/kjkrol/gram/plugins/underground/site"
)

// a tunnel's room and its width, metres of a figure's life: in the town a road's of two lanes
var (
	tunnelRoom  = float32(2.2)
	tunnelWidth = float32(4)
)

// a road's tunnel through 4 km of rock at most, climbing a metre in ten
var tunnelling = site.Rules{
	Longest: 40,
	Cover:   scale.Units(5),
	Grade:   0.1,
	Reshape: scale.Units(60),
}

// the town's tunnels as its map has them: shallower, steeper, through 3 km at most
func init() {
	if !here.town {
		return
	}
	tunnelRoom = 4.5
	tunnelWidth = 8
	tunnelling.Longest = 300
	tunnelling.Cover = scale.Units(1)
	tunnelling.Grade = 0.12
}

// tunnel is a road's tunnel, two wagons wide and nearly twice as wide as it is high
type tunnel struct{}

var _ underground.Figure = tunnel{}

func (tunnel) Define(p *underground.Plugin) {
	p.Types().Define(TunnelKind, underground.Spec{
		Allows: cell.Land,
		Width:  scale.Figure(tunnelWidth),
		Clear:  scale.Figure(tunnelRoom),
		Pace:   1,
	})
}
