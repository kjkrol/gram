package main

import (
	"image/color"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/underground"
	"github.com/kjkrol/gram/plugins/underground/site"
)

// where a mine may be dug: under at least half a metre of ground, the shaft no steeper than 15 in
// a hundred, its mouth's ground levelled by half a metre at the most
var digging = site.Rules{
	Longest: 12,
	Cover:   scale.Units(0.5),
	Grade:   0.15,
	Reshape: scale.Units(0.5),
}

// mine is a shaft 2.5 m wide and 2.6 m high, timbered, as in life, and a cave 7 m high
type mine struct{}

var _ underground.Figure = mine{}

var (
	rockColor   = color.RGBA{R: 94, G: 84, B: 74, A: 255}
	vaultColor  = color.RGBA{R: 80, G: 72, B: 64, A: 255}
	spoilColor  = color.RGBA{R: 74, G: 62, B: 48, A: 255}
	timberColor = color.RGBA{R: 112, G: 76, B: 44, A: 255}
)

func (mine) Define(p *underground.Plugin) {
	p.Types().Define(ShaftKind, underground.Spec{
		Allows: cell.Land,
		Width:  scale.Units(2.5),
		Clear:  scale.Units(2.6),
		Pace:   1,
		Look: underground.Look{
			Walls: rockColor,
			Floor: spoilColor,
			Roof:  vaultColor,
			Frame: timberColor,
			Rough: true,
			Sets:  true,
		},
	})
	p.Types().Chamber(CaveKind, underground.Spec{
		Allows: cell.Land,
		Clear:  scale.Units(7),
		Pace:   1,
		Look: underground.Look{
			Walls: rockColor,
			Floor: spoilColor,
			Roof:  vaultColor,
			Rough: true,
		},
	})
}
