// Command board-atlas is a small flat board drawn from the game's own atlas: a meadow with a pond,
// a road and a wood, each kind a sprite the game draws — striped grass, rippled water, a cobbled
// road, tree tops — rather than a plain colour, and a way of the road's kind laid as a band. Units
// walk from corner to corner over the road, Shift+P shows their routes; the same board could be drawn from the board's own
// atlas of the kinds' colours by giving WithRenderer nil. WASD, the wheel, a middle drag or the
// cursor at an edge move the camera; Space pauses, ] and [ set the tempo; K lists every key.
package main

import (
	"image/color"
	"time"

	"github.com/kjkrol/aabbworld/geom"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 48
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
	EntitySize   = 24
	UnitSpeed    = CellSize * 2
	UnitCount    = 4
)

// =========================== Game ===========================

type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — a board from the game's own atlas",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

type mainStage struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player // the one at this keyboard: the units are its
	shortcuts *players.Shortcuts
	unit      kind.Of[unitRow]
	stack     game.Scenes
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "board-atlas" }

func (s *mainStage) Stack() game.Scenes { return s.stack }

func (s *mainStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
		Entities: world.EntitiesCfg{MaxCount: 2 * UnitCount, MinSize: EntitySize, MaxSize: EntitySize},
		Camera:   camera.Config{ViewportWidth: ScreenWidth, ViewportHeight: ScreenHeight},
	})
	s.collision = collision.NewPlugin(s.world)
	if err := ctx.Use(s.collision); err != nil {
		return err
	}
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	// the kinds carry colours too, for a board drawn without an atlas of the game's
	s.board.CellKinds().Create(
		cell.Kind{Name: cell.Named("grass"), Cost: 2, Allows: cell.Land, Color: color.RGBA{R: 96, G: 150, B: 70, A: 255}},
		cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Water, Color: color.RGBA{R: 50, G: 100, B: 180, A: 255}},
		cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land, Color: color.RGBA{R: 160, G: 140, B: 110, A: 255}},
		cell.Kind{Name: cell.Named("wood"), Cost: 4, Allows: cell.Land, Veil: 0.6, Color: color.RGBA{R: 40, G: 100, B: 50, A: 255}},
	)
	if err := ctx.Use(s.board); err != nil {
		return err
	}
	s.selection = selection.NewPlugin(s.world)
	if err := ctx.Use(s.selection); err != nil {
		return err
	}
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	if err := ctx.Use(s.nav); err != nil {
		return err
	}
	s.players = players.NewPlugin(s.world, s.selection, s.nav)
	s.player = s.players.Local("player")
	if err := s.player.Bind(s.players.Defaults()...); err != nil {
		return err
	}
	if err := ctx.Use(s.players); err != nil {
		return err
	}
	s.defineKinds()

	main := &mainScene{stage: s, tps: ctx.TPS()}
	main.keys = players.SceneKeys{
		{Key: control.KeyK, Label: "Shortcuts; Esc closes them", Do: func(rt game.Runtime, c game.Composition) { s.shortcuts.Open(rt, c) }},
		{Key: control.KeyEscape, Shift: true, Label: "Quit", Do: func(rt game.Runtime, _ game.Composition) { rt.Quit() }},
		{Key: control.KeyB, Label: "Toggle the grid", Do: func(game.Runtime, game.Composition) { s.board.Res.Render.ToggleShowGridLines() }},
	}
	s.shortcuts = s.players.Shortcuts(main.keys)
	stack, err := game.NewStack(main, s.shortcuts)
	if err != nil {
		return err
	}
	s.stack = stack
	comp := stack.Composition()
	comp.Show(main.Name())
	return ctx.Track(comp)
}

func (s *mainStage) Restore(game.Persistence) (bool, error) { return false, nil }

type unitRow struct{ start, target cell.ID }

func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	order := comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	s.unit = units.Define("unit", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15},
		order, comp.Tagged(s.selection.Tags().Selectable, s.selection.Tags().Selected), comp.Tagged(s.player.Owner()))
}

// Spawn lays the meadow out: a pond in the middle, a wood in the north-east, a road round the pond
// from corner to corner as a way over the grass, and a unit in every corner bound for the opposite one.
func (s *mainStage) Spawn() error {
	brd := s.board.Res.Logic.Board
	at := func(x, y int) cell.ID { c, _ := brd.CellIndex(uint32(x), uint32(y)); return c }
	layout := board.Layout{Default: "grass"}
	for y := 5; y < 11; y++ {
		for x := 9; x < 15; x++ {
			layout.Cells = append(layout.Cells, cell.Entry{Kind: "water", Cell: at(x, y)})
		}
	}
	for y := 1; y < 6; y++ {
		for x := 16; x < 22; x++ {
			layout.Cells = append(layout.Cells, cell.Entry{Kind: "wood", Cell: at(x, y)})
		}
	}
	// the road: a ring round the pond, and spurs out to the corners
	ring := []cell.ID{}
	for x := 7; x <= 16; x++ {
		ring = append(ring, at(x, 3))
	}
	for y := 4; y <= 12; y++ {
		ring = append(ring, at(16, y))
	}
	for x := 15; x >= 7; x-- {
		ring = append(ring, at(x, 12))
	}
	for y := 11; y >= 4; y-- {
		ring = append(ring, at(7, y))
	}
	road, _ := s.board.CellKinds().Get("road")
	link := func(a, b cell.ID) {
		if bit, ok := grid.Link(brd, a, b); ok {
			w := layoutWay(&layout, a, road)
			w.Links |= bit
		}
		if bit, ok := grid.Link(brd, b, a); ok {
			w := layoutWay(&layout, b, road)
			w.Links |= bit
		}
	}
	for i := range ring {
		link(ring[i], ring[(i+1)%len(ring)])
	}
	corners := [][2]int{{1, 1}, {22, 1}, {22, 14}, {1, 14}}
	near := [][2]int{{7, 3}, {16, 3}, {16, 12}, {7, 12}}
	for k, c := range corners {
		x, y := c[0], c[1]
		for x != near[k][0] || y != near[k][1] {
			nx, ny := x, y
			if x != near[k][0] {
				nx += sign(near[k][0] - x)
			} else {
				ny += sign(near[k][1] - y)
			}
			link(at(x, y), at(nx, ny))
			x, y = nx, ny
		}
	}
	s.board.Seed(layout)
	var entries []kind.Entry
	for k, c := range corners {
		o := corners[(k+2)%4]
		entries = append(entries, s.unit.Entry(unitRow{start: at(c[0], c[1]), target: at(o[0], o[1])}))
	}
	s.world.Seed(entries...)
	return nil
}

func sign(v int) int {
	if v < 0 {
		return -1
	}
	return 1
}

// layoutWay is the way of kind across c in layout, added if none runs there yet.
func layoutWay(layout *board.Layout, c cell.ID, kind cell.Kind) *cell.WayEntry {
	for i := range layout.Ways {
		if layout.Ways[i].Cell == c {
			return &layout.Ways[i]
		}
	}
	layout.Ways = append(layout.Ways, cell.WayEntry{Kind: kind.Name.String(), Cell: c, Width: 14})
	return &layout.Ways[len(layout.Ways)-1]
}

func (s *mainStage) Update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	stage *mainStage
	keys  players.SceneKeys
	tps   *game.TPS
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage
	worldAtlas := render.NewAtlas()
	worldAtlas.RegisterAt(s.unit.SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 230, G: 80, B: 80, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	// the game's own atlas: a drawn sprite for every kind, at the kinds' SpriteIDs
	kinds := s.board.CellKinds()
	atlas := render.NewAtlas()
	for name, draw := range map[string]render.SpriteDrawer{
		"grass": striped(color.RGBA{R: 96, G: 150, B: 70, A: 255}, color.RGBA{R: 108, G: 162, B: 78, A: 255}),
		"water": rippled(color.RGBA{R: 50, G: 100, B: 180, A: 255}, color.RGBA{R: 80, G: 130, B: 205, A: 255}),
		"road":  cobbled(color.RGBA{R: 160, G: 140, B: 110, A: 255}, color.RGBA{R: 135, G: 118, B: 92, A: 255}),
		"wood":  treed(color.RGBA{R: 70, G: 120, B: 60, A: 255}, color.RGBA{R: 30, G: 85, B: 40, A: 255}),
	} {
		k, _ := kinds.Get(name)
		atlas.RegisterAt(k.SpriteID, CellSize, draw)
	}
	atlas.Close()
	s.board.WithRenderer(atlas)
	s.board.Res.Render.ShowGridLines = false

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	count := func() int { return s.world.Res.Telemetry.Count }
	layers := []render.Layer{render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())}
	return append(layers, render.NewTelemetryRenderer(&m.tps.Ticks, count).With(s.world.Clock().Reporter()), s.world.Clock().HUD())
}

func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.stage.players.EventHandler().HandleEvents(events)
	m.keys.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }

// =========================== Sprites ===========================

// striped is grass: a ground colour with lighter blades across it.
func striped(ground, blade color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		dst.Fill(ground)
		for i := 0; i < size; i += 6 {
			dst.FillRect(float32(i), float32((i*7)%size), 2, 5, blade)
		}
	}
}

// rippled is water: a colour with lighter ripples running across it.
func rippled(water, ripple color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		dst.Fill(water)
		for y := 4; y < size; y += 8 {
			dst.FillRect(float32((y/2)%size), float32(y), float32(size)/3, 1, ripple)
		}
	}
}

// cobbled is a road: stones set in mortar.
func cobbled(mortar, stone color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		dst.Fill(mortar)
		step := size / 4
		for y := 0; y < size; y += step {
			off := 0
			if (y/step)%2 == 1 {
				off = step / 2
			}
			for x := -off; x < size; x += step {
				dst.FillRect(float32(x+1), float32(y+1), float32(step-2), float32(step-2), stone)
			}
		}
	}
}

// treed is a wood: tree tops on the undergrowth.
func treed(under, top color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		dst.Fill(under)
		r := float32(size) / 5
		for _, c := range [][2]float32{{0.3, 0.3}, {0.7, 0.35}, {0.5, 0.7}, {0.2, 0.75}, {0.8, 0.8}} {
			dst.FillCircle(c[0]*float32(size), c[1]*float32(size), r, top)
		}
	}
}
