package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
	keys  players.SceneKeys
}

var unitColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}

var cellColors = map[string]color.RGBA{
	WaterCell:  {R: 40, G: 90, B: 170, A: 255},
	EarthCell:  {R: 110, G: 150, B: 75, A: 255},
	SandCell:   {R: 215, G: 195, B: 140, A: 255},
	RockCell:   {R: 130, G: 125, B: 120, A: 255},
	BrookCell:  {R: 90, G: 145, B: 205, A: 255},
	StreamCell: {R: 90, G: 145, B: 205, A: 255},
	RiverCell:  {R: 90, G: 145, B: 205, A: 255},
	RoadCell:   {R: 165, G: 135, B: 95, A: 255},
	ForestCell: {R: 30, G: 90, B: 45, A: 255},
}

var (
	deckColor   = color.RGBA{R: 115, G: 85, B: 55, A: 255}
	railColor   = color.RGBA{R: 70, G: 50, B: 35, A: 255}
	stoneColor  = color.RGBA{R: 150, G: 145, B: 135, A: 255}
	reasonColor = color.RGBA{R: 20, G: 20, B: 20, A: 180}
	pausedColor = color.RGBA{R: 120, G: 30, B: 30, A: 200}
	minimapRim  = color.RGBA{R: 30, G: 30, B: 30, A: 255}
)

var (
	snowyColors = map[string]color.RGBA{
		EarthCell:  {R: 232, G: 236, B: 235, A: 255},
		SandCell:   {R: 238, G: 236, B: 225, A: 255},
		RockCell:   {R: 205, G: 208, B: 212, A: 255},
		ForestCell: {R: 150, G: 185, B: 165, A: 255},
	}
	iceColor = color.RGBA{R: 175, G: 210, B: 230, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	a.board.WithAtmosphere(a.atmosphere.Read(0))
	a.vision.WithViews(world.Show(a.selection.Read(0).IsSelected))

	paused := func() bool { return a.world.Read(0).Clock().State().Paused }
	cam := a.cameras.Write().New(cameras.FromAbove, camera.Config{})
	return ui.Layers(
		ui.Image(render.NewFeed(cam, a.world.Read(0).Picture())).Input(a.players.Through(a.player)),
		ui.Drawer(a.world.Read(0).Clock().HUD()),
		ui.TopMiddle(beside(a.atmosphere.Bar(), paused)).Margin(8),
		ui.TopRight(ui.Layers(
			ui.Center(a.minimap.For(cam, a.player.ID).Masked(ui.Circle).Border(minimapRim, 2)).
				Size(220, 220),
			a.minimap.Compass(cam),
		)).Size(220+2*minimap.Overhang, 220+2*minimap.Overhang).Margin(16-minimap.Overhang),
		ui.BottomMiddle(a.bridges.Reason().Padding(6).Fill(reasonColor)).Margin(40),
		ui.Center(a.bridges.Window()),
	)
}

// beside is e with PAUSED on a red panel to its right while is says so: a slot as wide is kept
// on either side, so e stays centred as the word comes and goes
func beside(e *ui.Element, is func() bool) *ui.Element {
	const pad, gap = 6, 8
	w, _ := ui.DefaultTheme().Bold.Measure("PAUSED")
	slot := w + 2*pad + gap
	word := ui.HeadingOf(ui.While(is, "PAUSED")).Padding(pad).Fill(pausedColor)
	return ui.Columns(ui.Fixed(slot, ui.Layers()), ui.Fit(e), ui.Fixed(slot, ui.MiddleRight(word)))
}
