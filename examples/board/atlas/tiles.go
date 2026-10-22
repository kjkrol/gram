package main

import (
	"embed"
	"image/color"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

type grass struct{}

var _ board.Tile = grass{}

func (grass) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
}

func (grass) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(GrassCell, striped(grassColor, grassLitColor))
}

func striped(ground, blade color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		dst.Fill(ground)
		for i := 0; i < size; i += 6 {
			dst.FillRect(float32(i), float32((i*7)%size), 2, 5, blade)
		}
	}
}

//go:embed shimmer.wgsl
var shimmerFS embed.FS

// registered before the composer's shader compiles
var shimmerMaterial = render.RegisterMaterials(
	render.Files(shimmerFS, "shimmer.wgsl"), nil, "Shimmer")[0]

// water is a material, shimmering per pixel every frame
type water struct{}

var _ board.Tile = water{}

func (water) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(WaterCell, cell.Kind{Cost: 1, Allows: cell.Water})
}

func (water) Look(a *board.Atlas, _ *board.Plugin) { a.Add(WaterCell, shimmerMaterial) }

type road struct{}

var _ board.Tile = road{}

func (road) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

func (road) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(RoadCell, cobbled(roadColor, roadDimColor))
}

func cobbled(mortar, stone color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		dst.Fill(mortar)
		step := size / 4
		for y := 0; y < size; y += step {
			off := 0
			if (y/step)%2 == 1 {
				off = step / 2
			}
			for x := -off; x < size; x += step {
				dst.FillRect(float32(x+1), float32(y+1), float32(step-2), float32(step-2), stone)
			}
		}
	}
}

type wood struct{}

var _ board.Tile = wood{}

func (wood) Define(b *board.Plugin) {
	b.Read(0).CellKinds().Define(WoodCell, cell.Kind{Cost: 4, Allows: cell.Land, Veil: 0.6})
}

func (wood) Look(a *board.Atlas, _ *board.Plugin) {
	a.Add(WoodCell, treed(woodColor, woodDimColor))
}

func treed(under, top color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		dst.Fill(under)
		r := float32(size) / 5
		tops := [][2]float32{{0.3, 0.3}, {0.7, 0.35}, {0.5, 0.7}, {0.2, 0.75}, {0.8, 0.8}}
		for _, c := range tops {
			dst.FillCircle(c[0]*float32(size), c[1]*float32(size), r, top)
		}
	}
}
