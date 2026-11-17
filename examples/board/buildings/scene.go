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
	grassColor  = color.RGBA{R: 104, G: 148, B: 78, A: 255}
	bladeColor  = color.RGBA{R: 84, G: 128, B: 62, A: 255}
	cobbleColor = color.RGBA{R: 150, G: 142, B: 128, A: 255}
	jointColor  = color.RGBA{R: 122, G: 114, B: 102, A: 255}
	yardColor   = color.RGBA{R: 132, G: 124, B: 104, A: 255}
	heroColor   = color.RGBA{R: 230, G: 80, B: 80, A: 255}
	limeColor   = color.RGBA{R: 236, G: 230, B: 214, A: 255}
	ochreColor  = color.RGBA{R: 232, G: 204, B: 148, A: 255}
	brickColor  = color.RGBA{R: 176, G: 104, B: 84, A: 255}
	tileColor   = color.RGBA{R: 178, G: 84, B: 58, A: 255}
	slateColor  = color.RGBA{R: 96, G: 100, B: 112, A: 255}
	gravelColor = color.RGBA{R: 150, G: 146, B: 138, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	cam := a.cameras.Write().New(cameras.Isometrically,
		camera.Config{Zoom: StartZoom, Follow: HeroName})
	return ui.Layers(
		ui.Image(render.NewFeed(cam, a.world.Read(0).Picture())).
			Input(a.players.Through(a.player)),
		ui.Drawer(a.world.Read(0).Clock().HUD()),
	)
}
