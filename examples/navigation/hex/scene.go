package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
	keys  players.SceneKeys
}

var (
	redColor   = color.RGBA{R: 220, G: 90, B: 90, A: 255}
	blueColor  = color.RGBA{R: 90, G: 140, B: 220, A: 255}
	grassColor = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	waterColor = color.RGBA{R: 60, G: 100, B: 170, A: 255}
	deckColor  = color.RGBA{R: 115, G: 85, B: 55, A: 255}
	railColor  = color.RGBA{R: 70, G: 50, B: 35, A: 255}
	roadColor  = color.RGBA{R: 150, G: 130, B: 80, A: 255}
)

func (m *mainScene) picture() render.Picture {
	a := m.arena
	worldAtlas := a.world.NewAtlas()
	worldAtlas.Looks()
	worldAtlas.Close()
	a.world.WithRenderer(worldAtlas)

	boardAtlas := a.board.NewAtlas(2 * HexSize)
	boardAtlas.Looks()
	boardAtlas.Close()
	a.board.WithRenderer(boardAtlas)

	return render.NewComposer(
		a.board.Renderer(),
		a.world.Renderer(), a.collision.Renderer(),
		a.selection.Renderer(),
		a.nav.Renderer(),
		a.bridges.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	return ui.Layers(
		ui.Image(render.NewFeed(a.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(a.players.Through(a.player)),
		ui.BottomMiddle(a.bridges.Reason().Padding(6)).Margin(40),
		ui.Center(a.bridges.Window()),
	)
}
