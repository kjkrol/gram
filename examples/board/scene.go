package main

import (
	"image/color"
	"math"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type mainScene struct {
	arena *arena
	keys  players.SceneKeys
	tps   *game.TPS
}

var unitColor = color.RGBA{R: 230, G: 80, B: 80, A: 255}

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
	deckColor  = color.RGBA{R: 115, G: 85, B: 55, A: 255}
	railColor  = color.RGBA{R: 70, G: 50, B: 35, A: 255}
	stoneColor = color.RGBA{R: 150, G: 145, B: 135, A: 255}
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

func (m *mainScene) picture() render.Picture {
	s := m.arena
	s.board.WithAtmosphere(s.atmosphere)
	unitKind := kind.Named[unitRow](s.world.Kinds(), UnitKind)
	bridgeKind := kind.Named[crossingRow](s.world.Kinds(), BridgeKind)
	fordKind := kind.Named[crossingRow](s.world.Kinds(), FordKind)

	worldAtlas := s.world.NewAtlas()
	worldAtlas.Add(unitKind, EntitySize, render.Solid(unitColor))
	worldAtlas.Add(bridgeKind, CellSize, deck(0)).Facing(8, deck).Lying()
	worldAtlas.Add(fordKind, CellSize, stones(0)).Facing(8, stones).Lying()
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(CellSize)
	for name, c := range cellColors {
		boardAtlas.Add(name, render.Solid(c))
	}
	for name, c := range snowyColors {
		boardAtlas.Add(snowyCell(name), render.Solid(c))
	}
	boardAtlas.Add(IceCell, render.Solid(iceColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)
	s.board.Res.Render.ShowGridLines = false

	s.nav.WithRenderer(nil)
	s.vision.WithViews(render.Show(s.selection.IsSelected))
	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return render.NewComposer(
		s.atmosphere.Renderer(),
		s.board.Renderer(),
		s.world.Renderer(),
		s.atmosphere.Clouds(),
		s.vision.Renderer(),
		s.selection.Renderer(),
		s.nav.Renderer(),
		s.atmosphere.Precipitation(),
	)
}

func (m *mainScene) screen() *ui.Element {
	s := m.arena
	count := func() int { return s.world.Res.Telemetry.Count }
	return ui.Layers(
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).
			Input(s.players.Through(s.player)),
		ui.Layer(
			render.NewTelemetryRenderer(&m.tps.Ticks, count).
				With(s.world.Clock().Reporter(), s.atmosphere.Reporter()),
		),
		ui.Layer(s.world.Clock().HUD()),
	)
}

// deck is a bridge's deck running the way angle says, degrees against the clock from the east,
// from edge to edge of the cell — corner to corner slantwise — its rails along it.
func deck(angle float64) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		x0, y0, x1, y1 := across(angle, size)
		w := float32(size)
		dst.StrokeLine(x0, y0, x1, y1, w*0.5, deckColor)
		sin, cos := math.Sincos(angle * math.Pi / 180)
		ox, oy := float32(sin)*w*0.25, float32(cos)*w*0.25
		dst.StrokeLine(x0+ox, y0+oy, x1+ox, y1+oy, w*0.06, railColor)
		dst.StrokeLine(x0-ox, y0-oy, x1-ox, y1-oy, w*0.06, railColor)
	}
}

// stones are a ford's stepping stones the way angle says.
func stones(angle float64) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		x0, y0, x1, y1 := across(angle, size)
		for i := range 4 {
			t := (float32(i) + 0.5) / 4
			dst.FillCircle(x0+(x1-x0)*t, y0+(y1-y0)*t, float32(size)*0.09, stoneColor)
		}
	}
}

// across is the line through the middle of a sprite size wide the way angle says, edge to edge.
func across(angle float64, size int) (x0, y0, x1, y1 float32) {
	sin, cos := math.Sincos(angle * math.Pi / 180)
	half := float64(size) / 2
	reach := half / max(math.Abs(cos), math.Abs(sin))
	dx, dy := float32(cos*reach), float32(-sin*reach)
	m := float32(half)
	return m - dx, m - dy, m + dx, m + dy
}
