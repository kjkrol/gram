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
	palette = [8]color.RGBA{
		{R: 80, G: 120, B: 220, A: 255},
		{R: 90, G: 200, B: 110, A: 255},
		{R: 80, G: 200, B: 210, A: 255},
		{R: 150, G: 100, B: 220, A: 255},
		{R: 220, G: 210, B: 80, A: 255},
		{R: 230, G: 160, B: 60, A: 255},
		{R: 60, G: 160, B: 150, A: 255},
		{R: 220, G: 40, B: 40, A: 255},
	}
	backgroundColor = color.RGBA{R: 50, G: 50, B: 50, A: 255}
)

// a ball's shapes; the last colour of the palette is the flash of a hit
var shapes = [entityShapes]func(color.RGBA) render.SpriteDrawer{
	render.Solid,
	render.Border,
	render.Diamond,
	render.Cross,
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	feed := render.NewFeed(a.cameras.New(cameras.FromAbove, camera.Config{}),
		a.world.Read(0).Picture())
	return ui.Layers(
		ui.Blank().Fill(backgroundColor),
		ui.Image(feed).Input(a.players.Through(a.player)),
	)
}
