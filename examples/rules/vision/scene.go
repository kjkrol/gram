package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

var (
	preyColor       = color.RGBA{R: 120, G: 190, B: 255, A: 255}
	hunterColor     = color.RGBA{R: 225, G: 70, B: 70, A: 255}
	backgroundColor = color.RGBA{R: 40, G: 40, B: 46, A: 255}
)

func (m *mainScene) picture() render.Picture {
	s := m.arena
	preyKind := kind.Named[body](s.world.Kinds(), PreyKind)
	hunterKind := kind.Named[body](s.world.Kinds(), HunterKind)

	atlas := render.NewAtlas()
	atlas.Add(preyKind, RectSize, render.Solid(preyColor))
	atlas.Add(hunterKind, RectSize, render.Solid(hunterColor))
	atlas.Close()
	s.world.WithRenderer(atlas)
	s.vision.ShowCones()
	s.vision.WithRenderer(atlas)

	return render.NewComposer(s.vision.Renderer(), s.world.Renderer(), s.collision.Renderer())
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Layers(
		ui.Blank().Fill(backgroundColor),
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(s.players.Through(s.player)),
	)
}
