package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

var (
	palette = [entityColors]color.RGBA{
		{R: 80, G: 120, B: 220, A: 255},
		{R: 90, G: 200, B: 110, A: 255},
		{R: 80, G: 200, B: 210, A: 255},
		{R: 150, G: 100, B: 220, A: 255},
		{R: 220, G: 210, B: 80, A: 255},
		{R: 230, G: 160, B: 60, A: 255},
	}
	hitColor        = color.RGBA{R: 220, G: 40, B: 40, A: 255}
	backgroundColor = color.RGBA{R: 40, G: 42, B: 46, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	a.world.WithFog(backgroundColor)
	a.world.ShowSectors(world.SectorBands)
	feed := render.NewFeed(a.cameras.New(cameras.FromAbove, camera.Config{}), a.world.Picture())
	return ui.Layers(
		ui.Blank().Fill(backgroundColor),
		ui.Image(feed).Input(a.players.Through(a.player)),
	)
}
