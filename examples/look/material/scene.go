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
	grassColor = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	ashColor   = color.RGBA{R: 120, G: 115, B: 110, A: 255}
	sootColor  = color.RGBA{R: 70, G: 65, B: 60, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	feed := render.NewFeed(a.cameras.New(cameras.FromAbove, camera.Config{}), a.world.Picture())
	return ui.Image(feed).Input(a.players.Through(a.player))
}
