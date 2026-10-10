package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
	keys  players.SceneKeys
	tps   *game.TPS
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
	FordCell:   {R: 90, G: 145, B: 205, A: 255},
	RoadCell:   {R: 165, G: 135, B: 95, A: 255},
	BridgeCell: {R: 115, G: 85, B: 55, A: 255},
	ForestCell: {R: 30, G: 90, B: 45, A: 255},
}

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
	s := m.arena
	s.topography.
		Style(WaterCell, painter.Style{Under: true, Shine: 0.9}).
		Style(EarthCell, painter.Style{Spread: 0.3}).
		Style(SandCell, painter.Style{Spread: 0.35}).
		Style(RockCell, painter.Style{Spread: 0.25}).
		Style(BrookCell, painter.Style{Shine: 0.9, Flow: 30, MixWith: WaterCell}).
		Style(StreamCell, painter.Style{Shine: 0.9, Flow: 30, MixWith: WaterCell}).
		Style(RiverCell, painter.Style{Shine: 0.9, Flow: 22, MixWith: WaterCell}).
		Style(FordCell, painter.Style{Shine: 0.9, Flow: 22, MixWith: WaterCell})
	for _, name := range snowable {
		s.topography.Style(snowyCell(name), s.topography.StyleOf(name))
	}
	s.topography.Style(IceCell, painter.Style{Under: true, Shine: 0.3})
	s.topography.WithAtmosphere(s.atmosphere)
	unitKind := kind.Named[unitRow](s.world.Kinds(), UnitKind)
	plateauKind := kind.Named[unitRow](s.world.Kinds(), PlateauKind)
	rivalKind := kind.Named[unitRow](s.world.Kinds(), RivalKind)
	hawkKind := kind.Named[unitRow](s.world.Kinds(), HawkKind)

	worldAtlas := render.NewAtlas()
	worldAtlas.Add(unitKind, spritePx, render.Solid(playerColor))
	worldAtlas.Add(plateauKind, spritePx, render.Solid(playerColor))
	worldAtlas.Add(rivalKind, spritePx, render.Solid(rivalColor))
	worldAtlas.Add(hawkKind, spritePx, render.Diamond(hawkColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	for name, c := range cellColors {
		boardAtlas.Add(name, render.Solid(c))
	}
	for name, c := range snowyColors {
		boardAtlas.Add(snowyCell(name), render.Solid(c))
	}
	boardAtlas.Add(IceCell, render.Solid(iceColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)
	s.board.Res.Render.ShowGridLines = true
	s.nav.WithRenderer(nil)
	s.vision.WithViews(render.Show(s.selection.IsSelected)) // TODO: to jest rule a tu s.board.Res.Render.ShowGridLines = true jest flaga
	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.atmosphere.Renderer(), s.board.Renderer(), s.topography.Renderer(),
		s.world.Renderer(), s.vision.Renderer(), s.selection.Renderer(),
		s.nav.Renderer(), s.atmosphere.Precipitation())
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	count := func() int { return s.world.Res.Telemetry.Count }
	return ui.Layers(
		ui.Image(render.NewFeed(
			s.cameras.New(s.topography.Views(topography.Isometrically), camera.Config{}),
			m.picture())).Input(s.players.Through(s.player)),
		ui.Layer(
			render.NewTelemetryRenderer(&m.tps.Ticks, count).
				With(s.world.Clock().Reporter(), s.atmosphere.Reporter()),
		),
		ui.Layer(s.world.Clock().HUD()),
	)
}
