package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type gameplayScene struct{ arena *gameplayArena }

var (
	moverColor      = color.RGBA{R: 90, G: 200, B: 110, A: 255}
	backgroundColor = color.RGBA{R: 30, G: 30, B: 40, A: 255}
)

func (m *gameplayScene) picture() render.Picture {
	g := m.arena
	moverKind := kind.Named[world.Position](g.world.Kinds(), MoverKind)
	atlas := render.NewAtlas()
	atlas.Add(moverKind, EntitySize, render.Solid(moverColor))
	atlas.Close()
	g.world.WithRenderer(atlas)
	return render.NewComposer(g.world.Renderer())
}

func (m *gameplayScene) screen() *ui.Element {
	g := m.arena
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Blank().Fill(backgroundColor),
		ui.Image(render.NewFeed(g.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(g.players.Through(g.player)),
		ui.BottomLeft(ui.Label("P: panel, M: menu, F5: save, F9: load, K: keys")).Margin(8),
		ui.Center(ui.Window("PANEL",
			ui.Label("the world keeps ticking behind me"),
			ui.Button("Close (or P)", ui.Hide{Name: PanelElement}),
		)).Named(PanelElement).Hidden().Modal(),
	)
}
