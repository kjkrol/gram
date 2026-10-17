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
	witchColor        = color.RGBA{R: 200, G: 230, B: 255, A: 255}
	witchFrozenColor  = color.RGBA{R: 240, G: 248, B: 255, A: 255}
	walkerColor       = color.RGBA{R: 220, G: 90, B: 90, A: 255}
	walkerFrozenColor = color.RGBA{R: 235, G: 175, B: 175, A: 255}
	walkerHasteColor  = color.RGBA{R: 255, G: 150, B: 60, A: 255}
	witchHasteColor   = color.RGBA{R: 255, G: 210, B: 120, A: 255}
	boatHasteColor    = color.RGBA{R: 215, G: 140, B: 60, A: 255}
	boatColor         = color.RGBA{R: 140, G: 90, B: 40, A: 255}
	boatRimColor      = color.RGBA{R: 190, G: 220, B: 245, A: 255}
	grassColor        = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	snowColor         = color.RGBA{R: 235, G: 240, B: 245, A: 255}
	waterColor        = color.RGBA{R: 40, G: 90, B: 170, A: 255}
	iceColor          = color.RGBA{R: 170, G: 215, B: 240, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	feed := render.NewFeed(a.cameras.New(cameras.FromAbove, camera.Config{}), a.world.Picture())
	return ui.Image(feed).Input(a.players.Through(a.player))
}
