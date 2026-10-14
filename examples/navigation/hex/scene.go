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
	a := m.arena
	redKind := kind.Named[unitRow](a.world.Kinds(), RedKind)
	blueKind := kind.Named[unitRow](a.world.Kinds(), BlueKind)

	bridge := a.bridges.Types().Named(BridgeKind)

	worldAtlas := a.world.NewAtlas()
	worldAtlas.Add(redKind, EntitySize, render.Diamond(redColor))
	worldAtlas.Add(blueKind, EntitySize, render.Diamond(blueColor))
	worldAtlas.Add(bridge.Deck(), 2*HexSize, deck).Lying().Edges(railing)
	worldAtlas.Close()
	a.world.WithRenderer(worldAtlas)

	boardAtlas := a.board.NewAtlas(2 * HexSize)
	boardAtlas.Add(GrassCell, render.Hexagon(grassColor))
	boardAtlas.Add(WaterCell, render.Hexagon(waterColor))
	boardAtlas.Add(RoadCell, render.Hexagon(roadColor))
	boardAtlas.Close()
	a.board.WithRenderer(boardAtlas)

	a.nav.WithRenderer(nil)

	a.selection.WithRenderer(nil)
	a.bridges.WithRenderer(nil)

	return render.NewComposer(
		a.board.Renderer(),
		a.world.Renderer(), a.collision.Renderer(),
		a.selection.Renderer(),
		a.nav.Renderer(),
		a.bridges.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	return ui.Layers(
		ui.Image(render.NewFeed(a.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(a.players.Through(a.player)),
		ui.BottomMiddle(a.bridges.Reason().Padding(6)).Margin(40),
		ui.Center(a.bridges.Window()),
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
