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

var grassColor = color.RGBA{R: 60, G: 95, B: 60, A: 255}

func (m *mainScene) picture() render.Picture {
	a := m.arena
	heroKind := kind.Named[unitRow](a.world.Kinds(), HeroKind)

	worldAtlas := a.world.NewAtlas()
	worldAtlas.Add(heroKind, heroHeight, heroStanding(0)).
		Facing(heroWays, heroStanding).
		Animated(heroWalkFrames, gaitPeriod, heroPose(heroWalk)).WhileMoving()
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
