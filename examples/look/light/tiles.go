package main

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

type grass struct{}

var _ board.Tile = grass{}

func (grass) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

func (grass) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(GrassCell, func(dst *render.Canvas, size int) {
		dst.Fill(grassColor)
		for i := 0; i < size; i += 7 {
			dst.FillRect(float32(i), float32((i*5)%size), 2, 4, grassBladeColor)
		}
	})
}

// lava glows of its own: it takes no shade and no lamp's light
type lava struct{}

var _ board.Tile = lava{}

func (lava) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(LavaCell, cell.Kind{Cost: 3, Allows: cell.Land, Takes: render.Shines})
}

func (lava) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(LavaCell, func(dst *render.Canvas, size int) {
		dst.Fill(lavaColor)
		s := float32(size)
		dst.FillCircle(s*0.3, s*0.35, s*0.18, lavaHotColor)
		dst.FillCircle(s*0.7, s*0.7, s*0.14, lavaHotColor)
	})
}
