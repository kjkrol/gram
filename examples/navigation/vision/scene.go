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
	a := m.arena
	scoutKind := func(i int) kind.Of[unitRow] {
		return kind.Named[unitRow](a.world.Kinds(), scouts[i])
	}
	hawkKind := kind.Named[unitRow](a.world.Kinds(), HawkKind)

	worldAtlas := render.NewAtlas()
	for i := range scouts {
		worldAtlas.Add(scoutKind(i), EntitySize, render.Solid(unitColors[i]))
	}
	worldAtlas.Add(hawkKind, EntitySize, render.Diamond(hawkColor))
	worldAtlas.Close()
	a.world.WithRenderer(worldAtlas)

	boardAtlas := a.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Add(WallCell, render.Solid(wallColor))
	boardAtlas.Add(ForestCell, render.Solid(forestColor))
	boardAtlas.Add(RoadCell, render.Solid(roadColor))
	boardAtlas.Add(HillCell, render.Solid(hillColor))
	boardAtlas.Close()
	a.board.WithRenderer(boardAtlas)

	a.nav.WithRenderer(nil)

	a.vision.WithViews(render.Show(a.selection.IsSelected))
	a.vision.WithRenderer(nil)
	a.selection.WithRenderer(nil)

	return render.NewComposer(
		a.topography.Renderer(),
		a.board.Renderer(),
		a.world.Renderer(), a.collision.Renderer(),
		a.vision.Renderer(),
		a.selection.Renderer(),
		a.nav.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	return ui.Image(render.NewFeed(
		a.cameras.New(a.topography.Views(topography.FromAbove), camera.Config{}),
		m.picture())).Input(a.players.Through(a.player))
}
