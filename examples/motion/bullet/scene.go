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
	soldierColor  = color.RGBA{R: 90, G: 140, B: 230, A: 255}
	barrelColor   = color.RGBA{R: 230, G: 240, B: 255, A: 255}
	wandererColor = color.RGBA{R: 220, G: 90, B: 90, A: 255}
	paleColor     = color.RGBA{R: 235, G: 200, B: 200, A: 255}
	roundColor    = color.RGBA{R: 255, G: 230, B: 80, A: 255}
	grenadeColor  = color.RGBA{R: 50, G: 80, B: 50, A: 255}
	sparkColor    = color.RGBA{R: 255, G: 250, B: 200, A: 255}
	grassColor    = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	roadColor     = color.RGBA{R: 150, G: 130, B: 80, A: 255}
	waterColor    = color.RGBA{R: 40, G: 90, B: 170, A: 255}
	wallColor     = color.RGBA{R: 90, G: 90, B: 100, A: 255}
	lowWallColor  = color.RGBA{R: 150, G: 150, B: 160, A: 255}
)

func (m *mainScene) picture() render.Picture {
	a := m.arena
	soldierKind := kind.Named[unitRow](a.world.Kinds(), SoldierKind)
	wandererKind := kind.Named[unitRow](a.world.Kinds(), WandererKind)

	wounded := a.world.Effects().Named(WoundedEf)
	fuse := a.world.Effects().Named(FuseEf)

	// authored facing east: the barrel shows where F fires
	soldier := func(body, barrel color.RGBA) render.SpriteDrawer {
		return func(dst *render.Canvas, size int) {
			render.Diamond(body)(dst, size)
			render.Arrow(0, 3, barrel)(dst, size)
		}
	}
	worldAtlas := a.world.NewAtlas()
	worldAtlas.Add(soldierKind, EntitySize, soldier(soldierColor, barrelColor)).
		Under(wounded, soldier(paleColor, barrelColor)).
		Turning()
	worldAtlas.Add(wandererKind, EntitySize, render.Solid(wandererColor)).
		Under(wounded, render.Solid(paleColor))
	worldAtlas.Add(a.bullet.Shots().Named(RoundKind), 8, render.Arrow(0, 2, roundColor)).
		Turning() // the plugin keeps a shot's heading on its Vel.Dir
	worldAtlas.Add(a.bullet.Shots().Named(GrenadeKind), 8, render.Dot(3, grenadeColor)).
		Under(fuse, func(dst *render.Canvas, size int) {
			render.Dot(3, grenadeColor)(dst, size)
			render.Diamond(sparkColor)(dst, size)
		})
	worldAtlas.Close()
	a.world.WithRenderer(worldAtlas)

	boardAtlas := a.board.NewAtlas(CellSize)
	boardAtlas.Add(GrassCell, render.Solid(grassColor))
	boardAtlas.Add(RoadCell, render.Solid(roadColor))
	boardAtlas.Add(WaterCell, render.Solid(waterColor))
	boardAtlas.Add(WallCell, render.Solid(wallColor))
	boardAtlas.Add(LowWallCell, render.Solid(lowWallColor))
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
