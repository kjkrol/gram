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
	a := m.arena
	emberKind := kind.Named[unitRow](a.world.Kinds(), EmberKind)
	doused := a.world.Effects().Named(DousedEf)

	worldAtlas := a.world.NewAtlas()
	worldAtlas.Add(emberKind, EntitySize, emberMaterial).
		Under(doused, render.Dot(5, sootColor))
	worldAtlas.Close()
	a.world.WithRenderer(worldAtlas)

	boardAtlas := a.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
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
