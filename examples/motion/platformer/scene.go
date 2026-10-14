package main

import (
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

var (
	heroColor  = color.RGBA{R: 240, G: 170, B: 60, A: 255}
	dazedColor = color.RGBA{R: 200, G: 190, B: 170, A: 255}
	eyeColor   = color.RGBA{R: 40, G: 30, B: 30, A: 255}
	skyColor   = color.RGBA{R: 120, G: 170, B: 220, A: 255}
	rockColor  = color.RGBA{R: 90, G: 80, B: 75, A: 255}
	ledgeColor = color.RGBA{R: 150, G: 110, B: 70, A: 255}
)

func (m *mainScene) picture() render.Picture {
	a := m.arena
	hero := func(body color.RGBA) render.SpriteDrawer {
		return func(dst *render.Canvas, size int) {
			render.Solid(body)(dst, size)
			render.Dot(1, eyeColor)(dst, size)
		}
	}
	worldAtlas := a.world.NewAtlas()
	worldAtlas.Add(kind.Named[heroRow](a.world.Kinds(), HeroKind), HeroSize, hero(heroColor)).
		Under(a.world.Effects().Named(DazedEf), hero(dazedColor))
	worldAtlas.Close()
	a.world.WithRenderer(worldAtlas)

	boardAtlas := a.board.NewAtlas(CellSize)
	boardAtlas.Add(SkyCell, render.Solid(skyColor))
	boardAtlas.Add(RockCell, render.Solid(rockColor))
	boardAtlas.Add(LedgeCell, render.Solid(ledgeColor))
	boardAtlas.Close()
	a.board.WithRenderer(boardAtlas)

	return render.NewComposer(a.board.Renderer(), a.world.Renderer(), a.collision.Renderer())
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	view := a.cameras.New(cameras.TopDown(), camera.Config{Zoom: 2, Follow: HeroName})
	return ui.Image(render.NewFeed(view, m.picture())).Input(a.players.Through(a.player))
}
