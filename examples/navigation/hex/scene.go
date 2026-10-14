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
	waterColor = color.RGBA{R: 60, G: 100, B: 170, A: 255}
	deckColor  = color.RGBA{R: 115, G: 85, B: 55, A: 255}
	railColor  = color.RGBA{R: 70, G: 50, B: 35, A: 255}
	roadColor  = color.RGBA{R: 150, G: 130, B: 80, A: 255}
)

func (m *mainScene) picture() render.Picture {
	s := m.arena
	redKind := kind.Named[unitRow](s.world.Kinds(), RedKind)
	blueKind := kind.Named[unitRow](s.world.Kinds(), BlueKind)

	bridge := s.bridges.Types().Named(BridgeKind)

	worldAtlas := s.world.NewAtlas()
	worldAtlas.Add(redKind, EntitySize, render.Diamond(redColor))
	worldAtlas.Add(blueKind, EntitySize, render.Diamond(blueColor))
	worldAtlas.Add(bridge.Deck(), 2*HexSize, deck).Lying().Edges(railing)
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(2 * HexSize)
	boardAtlas.Add(GrassCell, render.Hexagon(grassColor))
	boardAtlas.Add(WaterCell, render.Hexagon(waterColor))
	boardAtlas.Add(RoadCell, render.Hexagon(roadColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)

	s.selection.WithRenderer(nil)
	s.bridges.WithRenderer(nil)

	return render.NewComposer(
		s.board.Renderer(),
		s.world.Renderer(), s.collision.Renderer(),
		s.selection.Renderer(),
		s.nav.Renderer(),
		s.bridges.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Layers(
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(s.players.Through(s.player)),
		ui.BottomMiddle(s.bridges.Reason().Padding(6)).Margin(40),
		ui.Center(s.bridges.Window()),
	)
}

// authored running east
func deck(dst *render.Canvas, size int) {
	w := float32(size)
	dst.Fill(deckColor)
	for i := range 8 {
		dst.FillRect(w*float32(i)/8, 0, w*0.02, w, railColor)
	}
}

func railing(dst *render.Canvas, size int) {
	w := float32(size)
	dst.FillRect(0, 0, w, w*0.2, railColor)
	for i := range 4 {
		dst.FillRect(w*(float32(i)+0.4)/4, 0, w*0.06, w, railColor)
	}
}
