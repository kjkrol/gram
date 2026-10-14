package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
	keys  players.SceneKeys
}

var (
	playerColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}
	rivalColor  = color.RGBA{R: 70, G: 110, B: 230, A: 255}
	hawkColor   = color.RGBA{R: 120, G: 130, B: 60, A: 255}
)

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
	pierColor   = color.RGBA{R: 160, G: 150, B: 135, A: 255}
	reasonColor = color.RGBA{R: 20, G: 20, B: 20, A: 180}
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

func (m *mainScene) picture() render.Picture {
	a := m.arena
	a.topography.
		Style(WaterCell, painter.Style{Under: true, Shine: 0.9}).
		Style(EarthCell, painter.Style{Spread: 0.3}).
		Style(SandCell, painter.Style{Spread: 0.35}).
		Style(RockCell, painter.Style{Spread: 0.25}).
		Style(BrookCell, painter.Style{Shine: 0.9, Flow: 30, MixWith: WaterCell}).
		Style(StreamCell, painter.Style{Shine: 0.9, Flow: 30, MixWith: WaterCell}).
		Style(RiverCell, painter.Style{Shine: 0.9, Flow: 22, MixWith: WaterCell})
	for _, name := range snowable() {
		a.topography.Style(snowyCell(name), a.topography.StyleOf(name))
	}
	a.topography.Style(IceCell, painter.Style{Under: true, Shine: 0.3})
	a.topography.WithAtmosphere(a.atmosphere)
	worldAtlas := a.world.NewAtlas()
	worldAtlas.Looks()
	worldAtlas.Close()
	a.world.WithRenderer(worldAtlas)

	boardAtlas := a.board.NewAtlas(CellSize)
	boardAtlas.Looks()
	boardAtlas.Close()
	a.board.WithRenderer(boardAtlas)
	a.vision.WithViews(render.Show(a.selection.IsSelected))

	return render.NewComposer(
		a.atmosphere.Renderer(), a.board.Renderer(), a.topography.Renderer(),
		a.world.Renderer(), a.collision.Renderer(), a.vision.Renderer(), a.selection.Renderer(),
		a.nav.Renderer(), a.bridges.Renderer(), a.atmosphere.Precipitation())
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	return ui.Layers(
		ui.Image(render.NewFeed(
			a.cameras.New(a.topography.Views(topography.Isometrically), camera.Config{}),
			m.picture())).Input(a.players.Through(a.player)),
		ui.Layer(a.world.Clock().HUD()),
		ui.BottomMiddle(a.bridges.Reason().Padding(6).Fill(reasonColor)).Margin(40),
		ui.Center(a.bridges.Window()),
	)
}
