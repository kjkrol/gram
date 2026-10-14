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
	calmColor       = color.RGBA{R: 70, G: 130, B: 220, A: 255}
	angryColor      = color.RGBA{R: 220, G: 60, B: 50, A: 255}
	ghostColor      = color.RGBA{R: 225, G: 230, B: 245, A: 200}
	eyeColor        = color.RGBA{A: 255}
	goldColor       = color.RGBA{R: 240, G: 200, B: 40, A: 255}
	backgroundColor = color.RGBA{R: 40, G: 44, B: 52, A: 255}
)

func (m *mainScene) picture() render.Picture {
	a := m.arena
	return render.NewComposer(a.world.Renderer(), a.collision.Renderer(), a.cameras.Renderer())
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	return ui.Layers(
		ui.Blank().Fill(backgroundColor),
		ui.Image(render.NewFeed(a.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(a.players.Through(a.player)),
	)
}
