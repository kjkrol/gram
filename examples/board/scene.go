package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/examples/island"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
	keys  players.SceneKeys
	tps   *game.TPS
}

var unitColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}

var (
	snowyColors = map[string]color.RGBA{
		island.EarthCell:  {R: 232, G: 236, B: 235, A: 255},
		island.SandCell:   {R: 238, G: 236, B: 225, A: 255},
		island.RockCell:   {R: 205, G: 208, B: 212, A: 255},
		island.ForestCell: {R: 150, G: 185, B: 165, A: 255},
	}
	iceColor = color.RGBA{R: 175, G: 210, B: 230, A: 255}
)

func (m *mainScene) picture() render.Picture {
	s := m.arena
	unitKind := kind.Named[unitRow](s.world.Kinds(), UnitKind)

	worldAtlas := render.NewAtlas()
	worldAtlas.Add(unitKind, EntitySize, render.Solid(unitColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	for name, c := range island.Colors {
		boardAtlas.Add(name, render.Solid(c))
	}
	for name, c := range snowyColors {
		boardAtlas.Add(snowyCell(name), render.Solid(c))
	}
	boardAtlas.Add(IceCell, render.Solid(iceColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)
	s.board.Res.Render.ShowGridLines = false

	s.nav.WithRenderer(nil)
	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.atmosphere.Renderer(),
		s.board.Renderer(),
		s.world.Renderer(),
		s.atmosphere.Clouds(),
		s.vision.Renderer(),
		s.selection.Renderer(),
		s.nav.Renderer(),
		s.atmosphere.Precipitation(),
	)
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	count := func() int { return s.world.Res.Telemetry.Count }
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(s.players.Through(s.player)),
		ui.Layer(
			render.NewTelemetryRenderer(&m.tps.Ticks, count).
				With(s.world.Clock().Reporter(), s.atmosphere.Reporter()),
		),
		ui.Layer(s.world.Clock().HUD()),
	)
}
