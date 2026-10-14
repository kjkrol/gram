package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

var (
	unitColors = []color.RGBA{
		{R: 220, G: 90, B: 90, A: 255},
		{R: 90, G: 140, B: 220, A: 255},
		{R: 230, G: 200, B: 80, A: 255},
	}
	hawkColor   = color.RGBA{R: 120, G: 130, B: 60, A: 255}
	grassColor  = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	wallColor   = color.RGBA{R: 40, G: 40, B: 40, A: 255}
	forestColor = color.RGBA{R: 25, G: 60, B: 30, A: 255}
	roadColor   = color.RGBA{R: 150, G: 130, B: 80, A: 255}
	hillColor   = color.RGBA{R: 110, G: 100, B: 70, A: 255}
)

func (m *mainScene) picture() render.Picture {
	s := m.arena
	scoutKind := func(i int) kind.Of[unitRow] {
		return kind.Named[unitRow](s.world.Kinds(), scouts[i])
	}
	hawkKind := kind.Named[unitRow](s.world.Kinds(), HawkKind)

	worldAtlas := render.NewAtlas()
	for i := range scouts {
		worldAtlas.Add(scoutKind(i), EntitySize, render.Solid(unitColors[i]))
	}
	worldAtlas.Add(hawkKind, EntitySize, render.Diamond(hawkColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Add(WallCell, render.Solid(wallColor))
	boardAtlas.Add(ForestCell, render.Solid(forestColor))
	boardAtlas.Add(RoadCell, render.Solid(roadColor))
	boardAtlas.Add(HillCell, render.Solid(hillColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)

	s.vision.WithViews(render.Show(s.selection.IsSelected))
	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.topography.Renderer(),
		s.board.Renderer(),
		s.world.Renderer(), s.collision.Renderer(),
		s.vision.Renderer(),
		s.selection.Renderer(),
		s.nav.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Image(render.NewFeed(
		s.cameras.New(s.topography.Views(topography.FromAbove), camera.Config{}),
		m.picture())).Input(s.players.Through(s.player))
}
