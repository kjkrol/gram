package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

var pausedColor = color.RGBA{R: 120, G: 30, B: 30, A: 200}

var cellColors = map[string]color.RGBA{
	WaterCell:  {R: 40, G: 90, B: 170, A: 255},
	SandCell:   {R: 215, G: 195, B: 140, A: 255},
	EarthCell:  {R: 110, G: 150, B: 75, A: 255},
	ForestCell: {R: 30, G: 90, B: 45, A: 255},
	RockCell:   {R: 130, G: 125, B: 120, A: 255},
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	a.topography.
		Style(WaterCell, painter.Style{Under: true, Shine: 0.9}).
		Style(SandCell, painter.Style{Spread: 0.35}).
		Style(EarthCell, painter.Style{Spread: 0.3}).
		Style(RockCell, painter.Style{Spread: 0.25})
	a.topography.WithAtmosphere(a.atmosphere)

	paused := func() bool { return a.world.Clock().State().Paused }
	cam := a.cameras.New(cameras.Isometrically, camera.Config{})
	return ui.Layers(
		ui.Image(render.NewFeed(cam, a.world.Picture())).Input(a.players.Through(a.player)),
		ui.Drawer(a.world.Clock().HUD()),
		ui.TopMiddle(beside(a.atmosphere.Bar(), paused)).Margin(8),
	)
}

// beside is e with PAUSED on a red panel to its right while is says so: a slot as wide is kept
// on either side, so e stays centred as the word comes and goes
func beside(e *ui.Element, is func() bool) *ui.Element {
	const pad, gap = 6, 8
	w, _ := ui.DefaultTheme().Bold.Measure("PAUSED")
	slot := float64(w) + 2*pad + gap
	word := ui.HeadingOf(ui.While(is, "PAUSED")).Padding(pad).Fill(pausedColor)
	return ui.Columns(ui.Fixed(slot, ui.Layers()), ui.Fit(e), ui.Fixed(slot, ui.MiddleRight(word)))
}
