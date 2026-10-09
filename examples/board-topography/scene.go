package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
	keys  players.SceneKeys
	tps   *game.TPS
}

// The scene's colours: the player's walkers and giants, the rival's, the hawk.
var (
	playerColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}
	rivalColor  = color.RGBA{R: 70, G: 110, B: 230, A: 255}
	hawkColor   = color.RGBA{R: 120, G: 130, B: 60, A: 255}
)

// picture dresses the units, the hawk and the island and hands the world's picture.
func (m *mainScene) picture() render.Picture {
	s := m.arena
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

	s.board.WithRenderer(nil)
	s.board.Res.Render.ShowGridLines = true
	s.nav.WithRenderer(nil)
	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.atmosphere.Renderer(), s.board.Renderer(), s.topography.Renderer(),
		s.world.Renderer(), s.vision.Renderer(), s.selection.Renderer(),
		s.nav.Renderer(), s.atmosphere.Precipitation())
}

// screen is the island through a camera of its own, the player's view, the telemetry and the clock over it.
func (m *mainScene) screen() *ui.Element {
	s := m.arena
	count := func() int { return s.world.Res.Telemetry.Count }
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Image(render.NewFeed(s.cameras.New(s.topography.Views(topography.Isometrically), camera.Config{}), m.picture())).Input(s.players.Through(s.player)),
		ui.Layer(render.NewTelemetryRenderer(&m.tps.Ticks, count).With(s.world.Clock().Reporter(), s.atmosphere.Reporter())),
		ui.Layer(s.world.Clock().HUD()),
	)
}
