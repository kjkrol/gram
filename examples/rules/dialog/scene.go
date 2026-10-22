package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct{ arena *arena }

var (
	meadowColor    = color.RGBA{R: 70, G: 110, B: 60, A: 255}
	travellerColor = color.RGBA{R: 230, G: 200, B: 90, A: 255}
	hostColor      = color.RGBA{R: 120, G: 160, B: 230, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	feed := render.NewFeed(a.cameras.New(cameras.FromAbove, camera.Config{}),
		a.world.Read(0).Picture())
	return ui.Layers(
		ui.Image(feed).Input(a.players.Through(a.player)),
		a.dialog.Window().Above().Offset(0, -6),
		ui.LabelOf(ui.Of(func(r *Regard) (string, bool) {
			chosen, ok := a.selection.Chosen(a.player.ID)
			return r.Stance(chosen), ok
		})).Where(ui.Tagged(a.selection.Tags().Hovered)).Below().Offset(0, 4),
	)
}
