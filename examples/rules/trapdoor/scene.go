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
	scoutColor    = color.RGBA{R: 90, G: 140, B: 230, A: 255}
	hasteColor    = color.RGBA{R: 170, G: 220, B: 255, A: 255}
	wandererColor = color.RGBA{R: 220, G: 150, B: 60, A: 255}
	grassColor    = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	boardsColor   = color.RGBA{R: 120, G: 90, B: 55, A: 255}
	pitColor      = color.RGBA{R: 15, G: 12, B: 20, A: 255}
)

func (m *mainScene) picture() render.Picture {
	a := m.arena
	scoutKind := kind.Named[unitRow](a.world.Kinds(), ScoutKind)
	wandererKind := kind.Named[unitRow](a.world.Kinds(), WandererKind)
	trapdoorKind := kind.Named[standingRow](a.world.Kinds(), TrapdoorKind)

	worldAtlas := a.world.NewAtlas()
	worldAtlas.Add(trapdoorKind, CellSize, render.Solid(boardsColor)).
		Under(a.world.Effects().Named(OpenEf), render.Solid(pitColor))
	worldAtlas.Add(scoutKind, EntitySize, render.Solid(scoutColor)).
		Under(a.world.Effects().Named(HasteEf), render.Solid(hasteColor))
	worldAtlas.Add(wandererKind, EntitySize, render.Diamond(wandererColor))
	worldAtlas.Close()
	a.world.WithRenderer(worldAtlas)

	boardAtlas := a.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Add(PitCell, render.Solid(pitColor))
	boardAtlas.Close()
	a.board.WithRenderer(boardAtlas)

	a.nav.WithRenderer(nil)
	a.selection.WithRenderer(nil)

	return render.NewComposer(
		a.board.Renderer(),
		a.world.Renderer(), a.collision.Renderer(),
		a.selection.Renderer(),
		a.nav.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	return ui.Image(render.NewFeed(a.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
		Input(a.players.Through(a.player))
}
