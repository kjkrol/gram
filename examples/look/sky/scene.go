package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

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
	a.atmosphere.ShowClouds(atmosphere.AllBodies)
	a.atmosphere.CastClouds(atmosphere.CloudShadows{Ground: true, Figures: true, High: true})

	cam := a.cameras.New(cameras.Isometrically, camera.Config{})
	return ui.Layers(
		ui.Image(render.NewFeed(cam, a.world.Picture())).Input(a.players.Through(a.player)),
		ui.Drawer(a.world.Clock().HUD()),
	)
}
