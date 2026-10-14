package main

import (
	"embed"
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

//go:embed shimmer.wgsl
var shimmerFS embed.FS

// registered before the composer's shader compiles
var shimmerMaterial = render.RegisterMaterials(
	render.Files(shimmerFS, "shimmer.wgsl"), nil, "Shimmer")[0]

var (
	unitColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}

	grassColor    = color.RGBA{R: 96, G: 150, B: 70, A: 255}
	grassLitColor = color.RGBA{R: 108, G: 162, B: 78, A: 255}
	waterColor    = color.RGBA{R: 50, G: 100, B: 180, A: 255}
	waterLitColor = color.RGBA{R: 80, G: 130, B: 205, A: 255}
	roadColor     = color.RGBA{R: 160, G: 140, B: 110, A: 255}
	roadDimColor  = color.RGBA{R: 135, G: 118, B: 92, A: 255}
	woodColor     = color.RGBA{R: 70, G: 120, B: 60, A: 255}
	woodDimColor  = color.RGBA{R: 30, G: 85, B: 40, A: 255}
)

func (m *mainScene) picture() render.Picture {
	s := m.arena
	unitKind := kind.Named[unitRow](s.world.Kinds(), UnitKind)
	worldAtlas := render.NewAtlas()
	worldAtlas.Add(unitKind, EntitySize, render.Diamond(unitColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	atlas := s.board.NewAtlas(CellSize)
	atlas.Add(GrassCell, striped(grassColor, grassLitColor))
	atlas.Add(WaterCell, shimmerMaterial)
	atlas.Add(RoadCell, cobbled(roadColor, roadDimColor))
	atlas.Add(WoodCell, treed(woodColor, woodDimColor))
	atlas.Close()
	s.board.WithRenderer(atlas)
	s.board.Res.Render.ShowGridLines = false

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.board.Renderer(),
		s.world.Renderer(),
		s.selection.Renderer(),
		s.nav.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Layers(
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(s.players.Through(s.player)),
		ui.Layer(s.world.Clock().HUD()),
	)
}

func striped(ground, blade color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		dst.Fill(ground)
		for i := 0; i < size; i += 6 {
			dst.FillRect(float32(i), float32((i*7)%size), 2, 5, blade)
		}
	}
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
