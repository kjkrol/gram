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

// grassColor is the meadow the bugs scuttle over.
var grassColor = color.RGBA{R: 60, G: 95, B: 60, A: 255}

// picture dresses the world and hands its picture.
func (m *mainScene) picture() render.Picture {
	s := m.arena
	bugKind := kind.Named[unitRow](s.world.Kinds(), BugKind)

	worldAtlas := s.world.NewAtlas()
	worldAtlas.Add(bugKind, EntitySize, bugFrame(0)).
		Animated(gaitFrames, gaitPeriod, bugFrame). // the gait, frame after frame of game time
		Turning()                                   // each frame turned the way the bug goes
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

// screen is the world through a camera of its own, the player's view.
func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).Input(s.players.Through(s.player))
}
