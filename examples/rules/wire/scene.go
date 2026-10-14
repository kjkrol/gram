package main

import (
	_ "embed"
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
	grassColor   = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	boardsColor  = color.RGBA{R: 120, G: 90, B: 55, A: 255}
	pitColor     = color.RGBA{R: 15, G: 12, B: 20, A: 255}
	plateColor   = color.RGBA{R: 160, G: 160, B: 170, A: 255}
	fenceColor   = color.RGBA{R: 85, G: 60, B: 40, A: 255}
	gateColor    = color.RGBA{R: 70, G: 75, B: 90, A: 255}
	gatewayColor = color.RGBA{R: 150, G: 130, B: 95, A: 255}
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
	)
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	feed := render.NewFeed(a.cameras.New(cameras.TopDown(), camera.Config{Follow: HeroName}),
		m.picture())
	return ui.Layers(
		ui.Image(feed).Input(a.players.Through(a.player)),
		ui.Label("Space: pull the lever").Under(a.world.Effects().Named(InReachEf)),
	)
}
