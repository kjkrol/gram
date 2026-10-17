package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

var (
	floorColor   = color.RGBA{R: 70, G: 80, B: 70, A: 255}
	wallColor    = color.RGBA{R: 30, G: 30, B: 35, A: 255}
	redColor     = color.RGBA{R: 220, G: 80, B: 80, A: 255}
	blueColor    = color.RGBA{R: 80, G: 130, B: 230, A: 255}
	dividerColor = color.RGBA{R: 240, G: 240, B: 240, A: 255}
)

type mainScene struct{ arena *arena }

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	a.board.ShowGrid(false)

	picture := a.world.Picture()
	red := render.NewFeed(
		a.cameras.New(cameras.FromAbove, camera.Config{Follow: RedBlock}),
		picture,
	)
	blue := render.NewFeed(
		a.cameras.New(cameras.FromAbove, camera.Config{Follow: BlueBlock}),
		picture,
	)
	minimap := render.NewFeed(a.cameras.New(cameras.FromAbove, camera.Config{Whole: true}), picture)
	return ui.Layers(
		ui.Columns(
			ui.Share(1, ui.Image(red).Input(a.players.Through(a.redPlayer))),
			ui.Fixed(2, ui.Blank().Fill(dividerColor)),
			ui.Share(1, ui.Image(blue).Input(a.players.Through(a.bluePlayer))),
		),
		ui.BottomMiddle(ui.Image(minimap).Border(dividerColor, 2)).
			Fraction(MinimapShare, 0).
			Margin(10),
	)
}
