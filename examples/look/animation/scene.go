package main

import (
	"github.com/kjkrol/ceremony"
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

var grassColor = color.RGBA{R: 60, G: 95, B: 60, A: 255}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	feed := render.NewFeed(a.cameras.New(cameras.FromAbove, camera.Config{}),
		a.world.Read(0).Picture())
	return ui.Layers(
		ui.Image(feed).Input(a.players.Through(a.player)),
		ui.Label("walking, braking").On(ceremony.Named(WalkerName)),
		ui.Label("bored: blinking, stamping, looking about").On(ceremony.Named(IdlerName)),
		ui.Label("pulling").On(ceremony.Named(PullerName)),
		ui.Label("swinging").On(ceremony.Named(LeverName)).Below(),
	)
}
