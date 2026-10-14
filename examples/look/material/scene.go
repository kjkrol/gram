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
)

func (m *mainScene) picture() render.Picture {
	a := m.arena
	return render.NewComposer(a.board.Renderer(), a.world.Renderer())
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	return ui.Image(render.NewFeed(a.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
		Input(a.players.Through(a.player))
}
