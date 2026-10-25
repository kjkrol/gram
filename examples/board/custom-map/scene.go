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

var unitColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	feed := render.NewFeed(a.cameras.Write().New(cameras.FromAbove, camera.Config{}),
		a.world.Read(0).Picture())
	return ui.Layers(
		ui.Image(feed).Input(a.players.Through(a.player)),
		ui.Drawer(a.world.Read(0).Clock().HUD()),
	)
}
