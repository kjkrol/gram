package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
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

// unitColor is the walkers': the one colour the scene names itself, the island's are its kinds'.
var unitColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}

// picture dresses the units and the island and hands the world's picture: the tiles and the
// bands, the units, the clouds' shadows over them all, then the cones, the overlays and the rain.
func (m *mainScene) picture() render.Picture {
	s := m.arena
	unitKind := kind.Named[unitRow](s.world.Kinds(), UnitKind)

	worldAtlas := render.NewAtlas()
	worldAtlas.Add(unitKind, EntitySize, render.Solid(unitColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	s.board.WithRenderer(nil) // the board's own atlas: every kind in its colour
	s.board.Res.Render.ShowGridLines = false

	s.nav.WithRenderer(nil)
	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(s.atmosphere.Renderer(), s.board.Renderer(), s.world.Renderer(), s.atmosphere.Clouds(), s.vision.Renderer(), s.selection.Renderer(), s.nav.Renderer(), s.atmosphere.Precipitation())
}

// screen is the world through a camera of its own, the player's view, the telemetry and the clock over it.
func (m *mainScene) screen() *ui.Element {
	s := m.arena
	count := func() int { return s.world.Res.Telemetry.Count }
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).Input(s.players.Through(s.player)),
		ui.Layer(render.NewTelemetryRenderer(&m.tps.Ticks, count).With(s.world.Clock().Reporter(), s.atmosphere.Reporter())),
		ui.Layer(s.world.Clock().HUD()),
	)
}
