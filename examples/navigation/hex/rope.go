package main

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

const (
	ropeWidth = HexSize // between its ropes wide enough for the wagon
	ropeLoad  = 200     // kg: it bears a scout, never the wagon
)

// rope is a footbridge of planks hung on ropes over the water: the scouts cross it, the wagon is
// too heavy for it
type rope struct{}

var _ bridges.Figure = rope{}

func (rope) Define(p *bridges.Plugin) {
	p.Types().Define(RopeKind, bridges.Spec{
		Allows: cell.Land,
		Load:   ropeLoad,
		Width:  ropeWidth,
		Pace:   1,
		Rail:   cell.Rail{Keeps: cell.Land},
	})
}

func (rope) Look(a *world.Atlas, p *bridges.Plugin) {
	a.Add(p.Types().Named(RopeKind).Deck(), 2*HexSize, planks).Lying().Edges(handrope)
}

// authored running east: slats across, a gap of water between each two
func planks(dst *render.Canvas, size int) {
	w := float32(size)
	for i := range 10 {
		dst.FillRect(w*(float32(i)+0.15)/10, 0, w*0.07, w, plankColor)
	}
	dst.FillRect(0, w*0.1, w, w*0.04, ropeColor)
	dst.FillRect(0, w*0.86, w, w*0.04, ropeColor)
}

// a rope sagging between two posts, seen from the side
func handrope(dst *render.Canvas, size int) {
	w := float32(size)
	dst.FillRect(0, 0, w*0.06, w, ropeColor)
	dst.FillRect(w*0.94, 0, w*0.06, w, ropeColor)
	dst.StrokeLine(0, w*0.1, w/2, w*0.3, w*0.05, ropeColor)
	dst.StrokeLine(w/2, w*0.3, w, w*0.1, w*0.05, ropeColor)
}
