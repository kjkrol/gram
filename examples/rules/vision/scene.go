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
	preyColor       = color.RGBA{R: 120, G: 190, B: 255, A: 255}
	hunterColor     = color.RGBA{R: 225, G: 70, B: 70, A: 255}
	backgroundColor = color.RGBA{R: 40, G: 40, B: 46, A: 255}
)

func (m *mainScene) picture() render.Picture {
	a := m.arena
	atlas := a.world.NewAtlas()
	atlas.Looks()
	atlas.Close()
	a.world.WithRenderer(atlas)
	a.vision.ShowCones()

	return render.NewComposer(a.vision.Renderer(), a.world.Renderer(), a.collision.Renderer())
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	return ui.Layers(
		ui.Blank().Fill(backgroundColor),
		ui.Image(render.NewFeed(a.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(a.players.Through(a.player)),
	)
}
