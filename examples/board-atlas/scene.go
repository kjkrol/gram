package main

import (
	"embed"
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
	tps   *game.TPS
}

//go:embed shimmer.wgsl
var shimmerFS embed.FS

// shimmerMaterial joins the composer's one shader as the package is set up, before it compiles.
var shimmerMaterial = render.RegisterMaterials(render.Files(shimmerFS, "shimmer.wgsl"), nil, "Shimmer")[0]

// The scene's colours: the unit and each kind's pair the drawn tiles shade between.
var (
	unitColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}

	grassColor, grassLitColor = color.RGBA{R: 96, G: 150, B: 70, A: 255}, color.RGBA{R: 108, G: 162, B: 78, A: 255}
	waterColor, waterLitColor = color.RGBA{R: 50, G: 100, B: 180, A: 255}, color.RGBA{R: 80, G: 130, B: 205, A: 255}
	roadColor, roadDimColor   = color.RGBA{R: 160, G: 140, B: 110, A: 255}, color.RGBA{R: 135, G: 118, B: 92, A: 255}
	woodColor, woodDimColor   = color.RGBA{R: 70, G: 120, B: 60, A: 255}, color.RGBA{R: 30, G: 85, B: 40, A: 255}
)

// picture dresses the unit and the board from the game's own atlas and hands the world's picture.
func (m *mainScene) picture() render.Picture {
	s := m.arena
	unitKind := kind.Named[unitRow](s.world.Kinds(), UnitKind)
	worldAtlas := render.NewAtlas()
	worldAtlas.Add(unitKind, EntitySize, render.Diamond(unitColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	// the game's own atlas: a drawn sprite for every kind, at the kinds' SpriteIDs
	atlas := s.board.NewAtlas(CellSize)
	atlas.Add(GrassCell, striped(grassColor, grassLitColor))
	atlas.Add(WaterCell, shimmerMaterial) // the water's look is a material: ripples run per pixel, across cells
	atlas.Add(RoadCell, cobbled(roadColor, roadDimColor))
	atlas.Add(WoodCell, treed(woodColor, woodDimColor))
	atlas.Close()
	s.board.WithRenderer(atlas)
	s.board.Res.Render.ShowGridLines = false

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())
}

// screen is the world through a camera of its own, the player's view, the telemetry and the clock over it.
func (m *mainScene) screen() *ui.Element {
	s := m.arena
	count := func() int { return s.world.Res.Telemetry.Count }
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).Input(s.players.Through(s.player)),
		ui.Layer(render.NewTelemetryRenderer(&m.tps.Ticks, count).With(s.world.Clock().Reporter())),
		ui.Layer(s.world.Clock().HUD()),
	)
}

// =========================== Sprites ===========================

// striped is grass: a ground colour with lighter blades across it.
func striped(ground, blade color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		dst.Fill(ground)
		for i := 0; i < size; i += 6 {
			dst.FillRect(float32(i), float32((i*7)%size), 2, 5, blade)
		}
	}
}

// cobbled is a road: stones set in mortar.
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

// treed is a wood: tree tops on the undergrowth.
func treed(under, top color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		dst.Fill(under)
		r := float32(size) / 5
		for _, c := range [][2]float32{{0.3, 0.3}, {0.7, 0.35}, {0.5, 0.7}, {0.2, 0.75}, {0.8, 0.8}} {
			dst.FillCircle(c[0]*float32(size), c[1]*float32(size), r, top)
		}
	}
}
