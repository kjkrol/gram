package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

var (
	floorColor   = color.RGBA{R: 70, G: 80, B: 70, A: 255}
	wallColor    = color.RGBA{R: 30, G: 30, B: 35, A: 255}
	redColor     = color.RGBA{R: 220, G: 80, B: 80, A: 255}
	blueColor    = color.RGBA{R: 80, G: 130, B: 230, A: 255}
	dividerColor = color.RGBA{R: 240, G: 240, B: 240, A: 255}
)

type mainScene struct{ arena *arena }

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	picture := m.picture()
	red := render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{Follow: RedBlock}), picture)
	blue := render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{Follow: BlueBlock}), picture)
	minimap := render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{Whole: true}), picture)
	return ui.Layers(
		ui.Columns(
			ui.Share(1, ui.Image(red).Input(s.players.Through(s.redPlayer))),   // the left half
			ui.Fixed(2, ui.Blank().Fill(dividerColor)),                         // the line between
			ui.Share(1, ui.Image(blue).Input(s.players.Through(s.bluePlayer))), // the right half
		),
		ui.BottomMiddle(ui.Image(minimap).Border(dividerColor, 2)).Fraction(MinimapShare, 0).Margin(10),
	)
}

func (m *mainScene) picture() render.Picture {
	s := m.arena
	redKind := kind.Named[block](s.world.Kinds(), RedKind)
	blueKind := kind.Named[block](s.world.Kinds(), BlueKind)
	worldAtlas := render.NewAtlas()
	worldAtlas.Add(redKind, BlockSize, render.Solid(redColor))
	worldAtlas.Add(blueKind, BlockSize, render.Solid(blueColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(FloorCell, render.Solid(floorColor))
	boardAtlas.Add(WallCell, render.Solid(wallColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)
	s.board.Res.Render.ShowGridLines = false

	return render.NewComposer(s.board.Renderer(), s.world.Renderer())
}
