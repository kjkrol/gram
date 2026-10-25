package main

import (
	"image/color"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// grass is the one ground the map lays; its shades are kinds of its own, never laid, whose
// sprites the hills dress its tiles in by height
type grass struct{}

var _ board.Tile = grass{}

var shadeColors = map[string][2]color.RGBA{
	GrassCell:  {{R: 96, G: 150, B: 70, A: 255}, {R: 108, G: 162, B: 78, A: 255}},
	RiseCell:   {{R: 120, G: 165, B: 80, A: 255}, {R: 134, G: 178, B: 90, A: 255}},
	SlopeCell:  {{R: 150, G: 170, B: 90, A: 255}, {R: 166, G: 184, B: 102, A: 255}},
	SummitCell: {{R: 184, G: 178, B: 120, A: 255}, {R: 200, G: 192, B: 136, A: 255}},
}

func (grass) Define(b *board.Plugin) {
	for _, name := range grassShades {
		b.Read(0).CellKinds().Define(name, cell.Kind{Cost: 2, Allows: cell.Land})
	}
}

func (grass) Look(a *board.Atlas, _ *board.Plugin) {
	for _, name := range grassShades {
		c := shadeColors[name]
		a.Add(name, striped(c[0], c[1]))
	}
}

func striped(ground, blade color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		dst.Fill(ground)
		for i := 0; i < size; i += 6 {
			dst.FillRect(float32(i), float32((i*7)%size), 2, 5, blade)
		}
	}
}
