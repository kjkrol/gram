package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

var (
	soldierColor  = color.RGBA{R: 90, G: 140, B: 230, A: 255}
	barrelColor   = color.RGBA{R: 230, G: 240, B: 255, A: 255}
	wandererColor = color.RGBA{R: 220, G: 90, B: 90, A: 255}
	paleColor     = color.RGBA{R: 235, G: 200, B: 200, A: 255}
	roundColor    = color.RGBA{R: 255, G: 230, B: 80, A: 255}
	grenadeColor  = color.RGBA{R: 50, G: 80, B: 50, A: 255}
	sparkColor    = color.RGBA{R: 255, G: 250, B: 200, A: 255}
	grassColor    = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	roadColor     = color.RGBA{R: 150, G: 130, B: 80, A: 255}
	waterColor    = color.RGBA{R: 40, G: 90, B: 170, A: 255}
	wallColor     = color.RGBA{R: 90, G: 90, B: 100, A: 255}
	lowWallColor  = color.RGBA{R: 150, G: 150, B: 160, A: 255}
)

func (m *mainScene) picture() render.Picture {
	a := m.arena
	worldAtlas := a.world.NewAtlas()
	worldAtlas.Looks()
	worldAtlas.Close()
	a.world.WithRenderer(worldAtlas)

	boardAtlas := a.board.NewAtlas(CellSize)
	boardAtlas.Looks()
	boardAtlas.Close()
	a.board.WithRenderer(boardAtlas)

	return render.NewComposer(
		a.board.Renderer(),
		a.world.Renderer(), a.collision.Renderer(),
		a.selection.Renderer(),
		a.nav.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	return ui.Image(render.NewFeed(a.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
		Input(a.players.Through(a.player))
}
