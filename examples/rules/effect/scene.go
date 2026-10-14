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
	witchColor        = color.RGBA{R: 200, G: 230, B: 255, A: 255}
	witchFrozenColor  = color.RGBA{R: 240, G: 248, B: 255, A: 255}
	walkerColor       = color.RGBA{R: 220, G: 90, B: 90, A: 255}
	walkerFrozenColor = color.RGBA{R: 235, G: 175, B: 175, A: 255}
	boatColor         = color.RGBA{R: 140, G: 90, B: 40, A: 255}
	boatRimColor      = color.RGBA{R: 190, G: 220, B: 245, A: 255}
	grassColor        = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	snowColor         = color.RGBA{R: 235, G: 240, B: 245, A: 255}
	waterColor        = color.RGBA{R: 40, G: 90, B: 170, A: 255}
	iceColor          = color.RGBA{R: 170, G: 215, B: 240, A: 255}
)

func (m *mainScene) picture() render.Picture {
	s := m.arena

	witchKind := kind.Named[unitRow](s.world.Kinds(), WitchKind)
	walkerKind := kind.Named[unitRow](s.world.Kinds(), WalkerKind)
	boatKind := kind.Named[unitRow](s.world.Kinds(), BoatKind)
	effects := s.world.Effects()
	frozen := effects.Named(FrozenEf)

	worldAtlas := s.world.NewAtlas()
	worldAtlas.Add(witchKind, EntitySize, render.Diamond(witchColor)).
		Under(frozen, render.Diamond(witchFrozenColor))
	worldAtlas.Add(walkerKind, EntitySize, render.Solid(walkerColor)).
		Under(frozen, render.Solid(walkerFrozenColor))
	worldAtlas.Add(boatKind, EntitySize, render.Solid(boatColor)).
		Under(frozen, func(dst *render.Canvas, size int) {
			render.Solid(boatColor)(dst, size)
			render.Border(boatRimColor)(dst, size)
		})
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor)).
		Under(effects.Named(FrostEf), render.Solid(snowColor))
	boardAtlas.Add(WaterCell, render.Solid(waterColor)).
		Under(effects.Named(IcedEf), render.Solid(iceColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.board.Renderer(),
		s.world.Renderer(), s.collision.Renderer(),
		s.selection.Renderer(),
		s.nav.Renderer(),
	)
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	return ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
		Input(s.players.Through(s.player))
}
