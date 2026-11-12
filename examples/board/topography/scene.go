package main

import (
	"image/color"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/examples/board/topography/hud"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
}

var (
	playerColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}
	rivalColor  = color.RGBA{R: 70, G: 110, B: 230, A: 255}
	hawkColor   = color.RGBA{R: 120, G: 130, B: 60, A: 255}
)

var cellColors = map[string]color.RGBA{
	WaterCell:      {R: 40, G: 90, B: 170, A: 255},
	EarthCell:      {R: 110, G: 150, B: 75, A: 255},
	SandCell:       {R: 215, G: 195, B: 140, A: 255},
	RockCell:       {R: 130, G: 125, B: 120, A: 255},
	BrookCell:      {R: 90, G: 145, B: 205, A: 255},
	StreamCell:     {R: 90, G: 145, B: 205, A: 255},
	RiverCell:      {R: 90, G: 145, B: 205, A: 255},
	RoadCell:       {R: 165, G: 135, B: 95, A: 255},
	LaurisilvaCell: {R: 24, G: 82, B: 46, A: 255},
	WoodlandCell:   {R: 58, G: 104, B: 56, A: 255},
	PinewoodCell:   {R: 34, G: 88, B: 72, A: 255},
	HeathCell:      {R: 116, G: 122, B: 86, A: 255},
	FarmlandCell:   {R: 142, G: 160, B: 82, A: 255},
	ParkCell:       {R: 104, G: 146, B: 72, A: 255},
	BuiltCell:      {R: 158, G: 146, B: 124, A: 255},
	PavedCell:      {R: 168, G: 160, B: 146, A: 255},
	AsphaltCell:    {R: 74, G: 74, B: 78, A: 255},
	BuildingCell:   {R: 150, G: 142, B: 130, A: 255},
}

var (
	deckColor  = color.RGBA{R: 115, G: 85, B: 55, A: 255}
	railColor  = color.RGBA{R: 70, G: 50, B: 35, A: 255}
	stoneColor = color.RGBA{R: 150, G: 145, B: 135, A: 255}
	pierColor  = color.RGBA{R: 160, G: 150, B: 135, A: 255}
	// the grass in the middle of a roundabout
	islandColor  = color.RGBA{R: 96, G: 132, B: 70, A: 255}
	bankColor    = color.RGBA{R: 112, G: 128, B: 84, A: 255}
	revetColor   = color.RGBA{R: 124, G: 118, B: 110, A: 255}
	walkColor    = color.RGBA{R: 172, G: 150, B: 118, A: 255}
	lanternGlow  = color.RGBA{R: 255, G: 200, B: 110, A: 255}
	lanternColor = render.Light{1, 0.75, 0.4}
	wallColor    = color.RGBA{R: 235, G: 230, B: 218, A: 255}
	roofColor    = color.RGBA{R: 190, G: 92, B: 60, A: 255}
	roofShade    = color.RGBA{R: 148, G: 66, B: 44, A: 255}
	ridgeColor   = color.RGBA{R: 214, G: 132, B: 96, A: 255}
	wagonColor   = color.RGBA{R: 120, G: 80, B: 45, A: 255}
	wheelColor   = color.RGBA{R: 50, G: 38, B: 28, A: 255}
)

var (
	snowyColors = map[string]color.RGBA{
		EarthCell:      {R: 232, G: 236, B: 235, A: 255},
		SandCell:       {R: 238, G: 236, B: 225, A: 255},
		RockCell:       {R: 205, G: 208, B: 212, A: 255},
		LaurisilvaCell: {R: 146, G: 182, B: 164, A: 255},
		WoodlandCell:   {R: 160, G: 190, B: 168, A: 255},
		PinewoodCell:   {R: 150, G: 184, B: 176, A: 255},
		HeathCell:      {R: 214, G: 218, B: 208, A: 255},
		FarmlandCell:   {R: 228, G: 232, B: 222, A: 255},
		ParkCell:       {R: 230, G: 234, B: 226, A: 255},
		BuiltCell:      {R: 226, G: 224, B: 220, A: 255},
		PavedCell:      {R: 228, G: 228, B: 226, A: 255},
	}
	iceColor = color.RGBA{R: 175, G: 210, B: 230, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	a.tunnels.WithCover(bankColor)
	d := details(a.look)
	a.topography.
		Style(WaterCell, painter.Style{Under: true, Shine: 0.9}).
		Style(EarthCell, painter.Style{Spread: 0.3, Detail: d[EarthCell]}).
		Style(SandCell, painter.Style{Spread: 0.35, Detail: d[SandCell]}).
		Style(RockCell, painter.Style{Spread: 0.25, Detail: d[RockCell]}).
		Style(LaurisilvaCell, painter.Style{Detail: d[LaurisilvaCell]}).
		Style(WoodlandCell, painter.Style{Detail: d[WoodlandCell]}).
		Style(PinewoodCell, painter.Style{Detail: d[PinewoodCell]}).
		Style(HeathCell, painter.Style{Spread: 0.3, Detail: d[HeathCell]}).
		Style(FarmlandCell, painter.Style{Spread: 0.3, Detail: d[FarmlandCell]}).
		Style(ParkCell, painter.Style{Spread: 0.3, Detail: d[FarmlandCell]}).
		Style(RoadCell, painter.Style{Detail: d[RoadCell]}).
		Style(BuiltCell, painter.Style{Spread: 0.3, Detail: d[RockCell]}).
		Style(PavedCell, painter.Style{Spread: 0.3, Detail: d[RockCell]}).
		Style(AsphaltCell, painter.Style{Ribbon: painter.Ribbon{
			Surface: cellColors[AsphaltCell],
			Island:  islandColor,
			Bank:    bankColor,
			Wall:    revetColor,
			Walk:    walkColor,
		}}).
		Style(BuildingCell, painter.Style{Detail: d[RockCell]}).
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
	a.buildings.WithAir(a.atmosphere.Read(0))
	a.bridges.WithAir(a.atmosphere.Read(0))
	a.vision.WithViews(world.Show(a.selection.Read(0).IsSelected))

	cam := a.cameras.Write().New(cameras.Isometrically, camera.Config{Zoom: here.zoom})
	return hud.Screen(hud.Deps{
		Feed: ui.Image(render.NewFeed(cam, a.world.Read(0).Picture())).
			Input(a.players.Through(a.player)),
		Minimap:    a.minimap.For(cam, a.player.ID),
		Compass:    a.minimap.Compass(cam),
		Clock:      a.world.Read(0).Clock().HUD(),
		Paused:     func() bool { return a.world.Read(0).Clock().State().Paused },
		Atmosphere: a.atmosphere,
		Reasons:    []*ui.Element{a.bridges.Reason(), a.tunnels.Reason()},
		Windows:    []*ui.Element{a.bridges.Window(), a.tunnels.Window()},
		Widgets:    a.hud == WidgetsHUD,
	})
}
