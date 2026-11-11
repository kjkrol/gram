package main

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/underground"
	"github.com/kjkrol/gram/plugins/underground/site"
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
