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
	guardColor    = color.RGBA{R: 90, G: 140, B: 220, A: 255}
	wandererColor = color.RGBA{R: 225, G: 215, B: 190, A: 255}
	grassColor    = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	wallColor     = color.RGBA{R: 40, G: 40, B: 40, A: 255}
)

func (m *mainScene) picture() render.Picture {
	s := m.arena
	worldAtlas := s.world.NewAtlas()
	worldAtlas.Add(kind.Named[guardRow](s.world.Kinds(), GuardKind), EntitySize,
		render.Solid(guardColor))
	worldAtlas.Add(kind.Named[wandererRow](s.world.Kinds(), WandererKind), EntitySize,
		render.Solid(wandererColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Add(WallCell, render.Solid(wallColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)
	s.nav.ShowRoutes(true)
	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.board.Renderer(),
		s.world.Renderer(),
		s.selection.Renderer(),
		s.nav.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
		Input(s.players.Through(s.player))
}
