package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
	keys  players.SceneKeys
}

var (
	redColor   = color.RGBA{R: 220, G: 90, B: 90, A: 255}
	blueColor  = color.RGBA{R: 90, G: 140, B: 220, A: 255}
	grassColor = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	wallColor  = color.RGBA{R: 40, G: 40, B: 40, A: 255}
	roadColor  = color.RGBA{R: 150, G: 130, B: 80, A: 255}
	holeColor  = color.RGBA{R: 10, G: 10, B: 30, A: 255}
)

func (m *mainScene) picture() render.Picture {
	a := m.arena
	redKind := kind.Named[unitRow](a.world.Kinds(), RedKind)
	blueKind := kind.Named[unitRow](a.world.Kinds(), BlueKind)

	worldAtlas := render.NewAtlas()
	worldAtlas.Add(redKind, EntitySize, render.Solid(redColor))
	worldAtlas.Add(blueKind, EntitySize, render.Solid(blueColor))
	worldAtlas.Close()
	a.world.WithRenderer(worldAtlas)

	boardAtlas := a.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Add(WallCell, render.Solid(wallColor))
	boardAtlas.Add(RoadCell, render.Solid(roadColor))
	boardAtlas.Add(HoleCell, render.Solid(holeColor))
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
