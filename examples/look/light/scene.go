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
	grassColor      = color.RGBA{R: 96, G: 150, B: 70, A: 255}
	grassBladeColor = color.RGBA{R: 108, G: 162, B: 78, A: 255}
	lavaColor       = color.RGBA{R: 225, G: 90, B: 30, A: 255}
	lavaHotColor    = color.RGBA{R: 255, G: 190, B: 60, A: 255}
	towerColor      = color.RGBA{R: 120, G: 110, B: 100, A: 255}
	towerTopColor   = color.RGBA{R: 150, G: 140, B: 128, A: 255}
	lanternGlow     = color.RGBA{R: 255, G: 210, B: 120, A: 255}
	lanternColor    = render.Light{1, 0.8, 0.45}
	walkerColor     = color.RGBA{R: 70, G: 110, B: 200, A: 255}
	heroColor       = color.RGBA{R: 230, G: 80, B: 80, A: 255}
)

// render3 is a light of three channels, for the sun's colours worked out by the hour
func render3(r, g, b float32) render.Light { return render.Light{r, g, b} }

func (m *mainScene) screen() *ui.Element {
	a := m.arena

	feed := render.NewFeed(a.cameras.New(cameras.FromAbove, camera.Config{}), a.world.Picture())
	return ui.Layers(
		ui.Image(feed).Input(a.players.Through(a.player)),
		ui.Drawer(a.world.Clock().HUD()),
	)
}
