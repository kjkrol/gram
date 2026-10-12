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
	walkerColor = color.RGBA{R: 225, G: 215, B: 190, A: 255}
	grassColor  = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	wallColor   = color.RGBA{R: 40, G: 40, B: 40, A: 255}
)

func (m *mainScene) picture() render.Picture {
	s := m.arena
	worldAtlas := render.NewAtlas()
	worldAtlas.Add(
		kind.Named[walkerRow](s.world.Kinds(), WalkerKind),
		EntitySize,
		render.Solid(walkerColor),
	)
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Add(WallCell, render.Solid(wallColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)
	s.board.ShowHolds(true)

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.board.Renderer(),
		s.board.HoldsRenderer(),
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
