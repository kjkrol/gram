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

// The scene's colours: the meadow and the calmed ward's ash.
var (
	grassColor = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	ashColor   = color.RGBA{R: 120, G: 115, B: 110, A: 255}
)

// picture dresses the world and hands its picture.
func (m *mainScene) picture() render.Picture {
	s := m.arena
	wardKind := kind.Named[wardRow](s.world.Kinds(), WardKind)
	calm := s.world.Effects().Named(CalmEf)

	worldAtlas := s.world.NewAtlas()
	worldAtlas.Add(wardKind, WardPx, wardMaterial). // the look is a material, not a sprite
							Under(calm, render.Dot(8, ashColor)) // calmed: a plain sprite until it wears off
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	return render.NewComposer(s.board.Renderer(), s.world.Renderer())
}

// screen is the world through a camera of its own, the player's view.
func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).Input(s.players.Through(s.player))
}
