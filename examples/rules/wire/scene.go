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
	porterColor   = color.RGBA{R: 150, G: 110, B: 200, A: 255}
	wandererColor = color.RGBA{R: 220, G: 150, B: 60, A: 255}
	grassColor    = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	boardsColor   = color.RGBA{R: 120, G: 90, B: 55, A: 255}
	pitColor      = color.RGBA{R: 15, G: 12, B: 20, A: 255}
	plateColor    = color.RGBA{R: 160, G: 160, B: 170, A: 255}
	leverColor    = color.RGBA{R: 200, G: 170, B: 60, A: 255}
	fenceColor    = color.RGBA{R: 85, G: 60, B: 40, A: 255}
	gateColor     = color.RGBA{R: 70, G: 75, B: 90, A: 255}
	gatewayColor  = color.RGBA{R: 150, G: 130, B: 95, A: 255}
)

func (m *mainScene) picture() render.Picture {
	s := m.arena
	scoutKind := kind.Named[unitRow](s.world.Kinds(), ScoutKind)
	porterKind := kind.Named[unitRow](s.world.Kinds(), PorterKind)
	wandererKind := kind.Named[unitRow](s.world.Kinds(), WandererKind)
	standing := func(name string) kind.Of[standingRow] {
		return kind.Named[standingRow](s.world.Kinds(), name)
	}
	fx := s.world.Effects()

	worldAtlas := render.NewAtlas()
	worldAtlas.Add(standing(TrapdoorKind), CellSize, render.Solid(boardsColor)).
		Under(fx.Named(OpenEf), render.Solid(pitColor))
	worldAtlas.Add(standing(GateKind), CellSize, render.Solid(gateColor)).
		Under(fx.Named(AjarEf), render.Solid(gatewayColor))
	worldAtlas.Add(standing(PlateKind), EntitySize, render.Solid(plateColor))
	worldAtlas.Add(standing(LeverKind), EntitySize, render.Solid(leverColor))
	worldAtlas.Add(scoutKind, EntitySize, render.Solid(scoutColor)).
		Under(fx.Named(HasteEf), render.Solid(hasteColor))
	worldAtlas.Add(porterKind, EntitySize, render.Solid(porterColor))
	worldAtlas.Add(wandererKind, EntitySize, render.Diamond(wandererColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Add(PitCell, render.Solid(pitColor))
	boardAtlas.Add(FenceCell, render.Solid(fenceColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.board.Renderer(),
		s.world.Renderer(),
		s.selection.Renderer(),
		s.nav.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
		Input(s.players.Through(s.player))
}
