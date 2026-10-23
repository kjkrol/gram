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
	guardColor    = color.RGBA{R: 90, G: 140, B: 220, A: 255}
	wandererColor = color.RGBA{R: 225, G: 215, B: 190, A: 255}
	grassColor    = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	wallColor     = color.RGBA{R: 40, G: 40, B: 40, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena

	feed := render.NewFeed(a.cameras.Write().New(cameras.FromAbove, camera.Config{}),
		a.world.Read(0).Picture())
	return ui.Image(feed).Input(a.players.Through(a.player))
}
