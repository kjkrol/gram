package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/examples/board/topography/hud"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
	keys  players.SceneKeys
}

var (
	playerColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}
	rivalColor  = color.RGBA{R: 70, G: 110, B: 230, A: 255}
	hawkColor   = color.RGBA{R: 120, G: 130, B: 60, A: 255}
)

var cellColors = map[string]color.RGBA{
	WaterCell:  {R: 40, G: 90, B: 170, A: 255},
	EarthCell:  {R: 110, G: 150, B: 75, A: 255},
	SandCell:   {R: 215, G: 195, B: 140, A: 255},
	RockCell:   {R: 130, G: 125, B: 120, A: 255},
	BrookCell:  {R: 90, G: 145, B: 205, A: 255},
	StreamCell: {R: 90, G: 145, B: 205, A: 255},
	RiverCell:  {R: 90, G: 145, B: 205, A: 255},
	RoadCell:   {R: 165, G: 135, B: 95, A: 255},
	ForestCell: {R: 30, G: 90, B: 45, A: 255},
}

var (
	deckColor    = color.RGBA{R: 115, G: 85, B: 55, A: 255}
	railColor    = color.RGBA{R: 70, G: 50, B: 35, A: 255}
	stoneColor   = color.RGBA{R: 150, G: 145, B: 135, A: 255}
	pierColor    = color.RGBA{R: 160, G: 150, B: 135, A: 255}
	lanternGlow  = color.RGBA{R: 255, G: 200, B: 110, A: 255}
	lanternColor = render.Light{1, 0.75, 0.4}
)

var (
	snowyColors = map[string]color.RGBA{
		EarthCell:  {R: 232, G: 236, B: 235, A: 255},
		SandCell:   {R: 238, G: 236, B: 225, A: 255},
		RockCell:   {R: 205, G: 208, B: 212, A: 255},
		ForestCell: {R: 150, G: 185, B: 165, A: 255},
	}
	iceColor = color.RGBA{R: 175, G: 210, B: 230, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	d := details(a.look)
	a.topography.
		Style(WaterCell, painter.Style{Under: true, Shine: 0.9}).
		Style(EarthCell, painter.Style{Spread: 0.3, Detail: d[EarthCell]}).
		Style(SandCell, painter.Style{Spread: 0.35, Detail: d[SandCell]}).
		Style(RockCell, painter.Style{Spread: 0.25, Detail: d[RockCell]}).
		Style(ForestCell, painter.Style{Detail: d[ForestCell]}).
		Style(RoadCell, painter.Style{Detail: d[RoadCell]}).
		Style(BrookCell, painter.Style{Shine: 0.9, Flow: 30, MixWith: WaterCell}).
		Style(StreamCell, painter.Style{Shine: 0.9, Flow: 30, MixWith: WaterCell}).
		Style(RiverCell, painter.Style{Shine: 0.9, Flow: 22, MixWith: WaterCell}).
		WithShading(shading(a.look))
	for _, name := range snowable() {
		s := a.topography.Read(0).StyleOf(name)
		s.Detail = d[snowyCell(name)]
		a.topography.Style(snowyCell(name), s)
	}
	a.topography.Style(IceCell, painter.Style{Under: true, Shine: 0.3})
	a.topography.WithAtmosphere(a.atmosphere.Read(0))
	a.vision.WithViews(render.Show(a.selection.Read(0).IsSelected))

	cam := a.cameras.Write().New(cameras.Isometrically, camera.Config{})
	return hud.Screen(hud.Deps{
		Feed: ui.Image(render.NewFeed(cam, a.world.Read(0).Picture())).
			Input(a.players.Through(a.player)),
		Minimap:    a.minimap.For(cam, a.player.ID),
		Compass:    a.minimap.Compass(cam),
		Clock:      a.world.Read(0).Clock().HUD(),
		Paused:     func() bool { return a.world.Read(0).Clock().State().Paused },
		Atmosphere: a.atmosphere,
		Reason:     a.bridges.Reason(),
		Window:     a.bridges.Window(),
		Widgets:    a.hud == WidgetsHUD,
	})
}
