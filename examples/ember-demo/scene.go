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
	grassColor = color.RGBA{R: 55, G: 80, B: 55, A: 255}
	sootColor  = color.RGBA{R: 70, G: 65, B: 60, A: 255}
)

func (m *mainScene) picture() render.Picture {
	s := m.arena
	emberKind := kind.Named[unitRow](s.world.Kinds(), EmberKind)
	doused := s.world.Effects().Named(DousedEf)

	worldAtlas := s.world.NewAtlas()
	worldAtlas.Add(emberKind, EntitySize, emberMaterial).
		Under(doused, render.Dot(5, sootColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).Input(s.players.Through(s.player))
}
