package main

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

func (m *mainScene) picture() render.Picture {
	s := m.arena
	scoutKind := func(i int) kind.Of[unitRow] { return kind.Named[unitRow](s.world.Kinds(), scouts[i]) }
	hawkKind := kind.Named[unitRow](s.world.Kinds(), HawkKind)

	worldAtlas := render.NewAtlas()
	for i := range scouts {
		worldAtlas.Add(scoutKind(i), EntitySize, render.Diamond(unitColors[i]))
	}
	worldAtlas.Add(hawkKind, EntitySize, render.Diamond(hawkColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(hexSprite)
	boardAtlas.Add(GrassCell, render.Hexagon(grassColor))
	boardAtlas.Add(WallCell, render.Hexagon(wallColor))
	boardAtlas.Add(ForestCell, render.Hexagon(forestColor))
	boardAtlas.Add(RoadCell, render.Hexagon(roadColor))
	boardAtlas.Add(HillCell, render.Hexagon(hillColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)

	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(s.topography.Renderer(), s.board.Renderer(), s.world.Renderer(), s.vision.Renderer(), s.selection.Renderer(), s.nav.Renderer())
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Image(render.NewFeed(s.cameras.New(s.topography.Views(topography.FromAbove), camera.Config{}), m.picture())).Input(s.players.Through(s.player))
}

// TODO: ta stale sa zupelnie nie na swoim miejscu
const (
	wallCol = 10
	gapRow  = 6

	roadLeft, roadRight uint32 = 2, GridWidth - 3
	roadTop, roadBottom uint32 = 0, GridHeight - 1
)

// hillHeight is how high the hill stands over the grass.
const hillHeight = 12
