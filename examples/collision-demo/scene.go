package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
	tps   *game.TPS
}

// The scene's colours: a palette of the bodies' colours — the one past entityColors is the
// hit's overlay — and the backdrop.
var (
	palette = [8]color.RGBA{
		{R: 80, G: 120, B: 220, A: 255},
		{R: 90, G: 200, B: 110, A: 255},
		{R: 80, G: 200, B: 210, A: 255},
		{R: 150, G: 100, B: 220, A: 255},
		{R: 220, G: 210, B: 80, A: 255},
		{R: 230, G: 160, B: 60, A: 255},
		{R: 60, G: 160, B: 150, A: 255},
		{R: 220, G: 40, B: 40, A: 255},
	}
	backgroundColor = color.RGBA{R: 50, G: 50, B: 50, A: 255}
)

// picture dresses the world and hands its picture.
func (m *mainScene) picture() render.Picture {
	s := m.arena

	atlas := s.world.NewAtlas()
	hit := s.world.Effects().Named(HitEf)
	flash := palette[entityColors]
	shapes := [entityShapes]func(color.RGBA) render.SpriteDrawer{render.Solid, render.Border, render.Diamond, render.Cross}
	bodyKindOf := func(c, sh int) kind.Of[body] { return kind.Named[body](s.world.Kinds(), bodyKind(c, sh)) }
	for ci, c := range palette[:entityColors] {
		for si, shape := range shapes {
			atlas.Add(bodyKindOf(ci, si), int(RectSize), shape(c)).
				Under(hit, func(dst *render.Canvas, size int) { // struck: its own shape, flashed
					shapes[si](flash)(dst, size)
				})
		}
	}
	atlas.Close()
	s.world.WithRenderer(atlas)

	return render.NewComposer(s.world.Renderer())
}

// screen is the world through a camera of its own, the player's view, on its backdrop, a telemetry line over it.
func (m *mainScene) screen() *ui.Element {
	s := m.arena
	entityCount := func() int { return s.world.Res.Telemetry.Count }
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Blank().Fill(backgroundColor),
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).Input(s.players.Through(s.player)),
		ui.Layer(render.NewTelemetryRenderer(&m.tps.Ticks, entityCount).With(s.collisionStats.Reporter(&m.tps.Ticks))),
	)
}
