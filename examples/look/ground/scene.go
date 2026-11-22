package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/mosaic"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

var skyColor = color.RGBA{R: 150, G: 178, B: 204, A: 255}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	a.ground.SetLook(mosaic.DefaultLook)
	cam := a.cameras.Write().New(cameras.Isometrically, camera.Config{Zoom: StartZoom})
	return ui.Layers(
		ui.Blank().Fill(skyColor),
		ui.Image(render.NewFeed(cam, a.world.Read(0).Picture())).
			Input(a.players.Through(a.player)),
	)
}
