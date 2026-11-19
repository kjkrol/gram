package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

var (
	grassColor   = color.RGBA{R: 104, G: 148, B: 78, A: 255}
	bladeColor   = color.RGBA{R: 84, G: 128, B: 62, A: 255}
	screeColor   = color.RGBA{R: 128, G: 138, B: 104, A: 255}
	stoneColor   = color.RGBA{R: 150, G: 146, B: 138, A: 255}
	earthColor   = color.RGBA{R: 140, G: 112, B: 80, A: 255}
	rutColor     = color.RGBA{R: 112, G: 88, B: 62, A: 255}
	skinColor    = color.RGBA{R: 226, G: 182, B: 150, A: 255}
	helmetColor  = color.RGBA{R: 238, G: 196, B: 40, A: 255}
	overallColor = color.RGBA{R: 70, G: 92, B: 150, A: 255}
)

// the afternoon sun over the hill, from the south-west
var afternoon = light.Distant{
	Dir:      [3]float32{-0.5, 0.6, 0.75},
	Strength: 0.7,
	Ambient:  0.35,
	Color:    [3]float32{1, 0.95, 0.85},
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	a.light.WithDistant(afternoon)
	cam := a.cameras.Write().New(cameras.Isometrically,
		camera.Config{Zoom: StartZoom, Follow: MinerName})
	return ui.Layers(
		ui.Image(render.NewFeed(cam, a.world.Read(0).Picture())).
			Input(a.players.Through(a.player)),
		ui.Drawer(a.world.Read(0).Clock().HUD()),
	)
}
