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
	unitColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}

	grassColor    = color.RGBA{R: 96, G: 150, B: 70, A: 255}
	grassLitColor = color.RGBA{R: 108, G: 162, B: 78, A: 255}
	waterColor    = color.RGBA{R: 50, G: 100, B: 180, A: 255}
	waterLitColor = color.RGBA{R: 80, G: 130, B: 205, A: 255}
	roadColor     = color.RGBA{R: 160, G: 140, B: 110, A: 255}
	roadDimColor  = color.RGBA{R: 135, G: 118, B: 92, A: 255}
	woodColor     = color.RGBA{R: 70, G: 120, B: 60, A: 255}
	woodDimColor  = color.RGBA{R: 30, G: 85, B: 40, A: 255}
	gridColor     = color.RGBA{R: 240, G: 230, B: 190, A: 150}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena

	feed := render.NewFeed(a.cameras.New(cameras.FromAbove, camera.Config{}),
		a.world.Read(0).Picture())
	return ui.Layers(
		ui.Image(feed).Input(a.players.Through(a.player)),
		ui.Drawer(a.world.Read(0).Clock().HUD()),
	)
}
