package main

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const deckWidth = 23

// bridge is a graded deck over the water, railed
type bridge struct{}

var _ bridges.Figure = bridge{}

func (bridge) Define(p *bridges.Plugin) {
	p.Types().Define(BridgeKind, bridges.Spec{
		Allows: cell.Land,
		Width:  deckWidth,
		Pace:   1,
		Rail:   cell.Rail{Keeps: cell.Land},
	})
}

func (bridge) Look(a *world.Atlas, p *bridges.Plugin) {
	a.Add(p.Types().Named(BridgeKind).Deck(), 2*HexSize, deck).Lying().Edges(railing)
}

// authored running east
func deck(dst *render.Canvas, size int) {
	w := float32(size)
	dst.Fill(deckColor)
	for i := range 8 {
		dst.FillRect(w*float32(i)/8, 0, w*0.02, w, railColor)
	}
}

func railing(dst *render.Canvas, size int) {
	w := float32(size)
	dst.FillRect(0, 0, w, w*0.2, railColor)
	for i := range 4 {
		dst.FillRect(w*(float32(i)+0.4)/4, 0, w*0.06, w, railColor)
	}
}
