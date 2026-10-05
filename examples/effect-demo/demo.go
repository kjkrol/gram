// Command effect-demo is an ice witch: an entity under orders that turns the ground round her
// feet into snow and the water into ice — the terrain is hers to write, as far as her power
// reaches, and it thaws some seconds after she has gone. She is fast on her own snow; a walker
// can cross the lake on her trail while it lasts — slipping, so it brakes badly and may not stop
// before ice that melts ahead of it — and a boat, whose brakes are weak, sails onto the ice it saw
// coming and is frozen in — still, immovable, in its own frozen look, as each kind has one — until
// the ice melts. Everything temporary here is an effect; who obeys which rule is a role its kind
// plays; how a kind looks under an effect is the effect's look of its sprite, drawn in the atlas.
package main

import (
	"image/color"
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
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
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
	EntitySize   = 22
	UnitSpeed    = CellSize * 2
	MaxEntCount  = 16 // the units; cell entities do not count

	lakeLeft, lakeRight uint32 = 8, 15
	lakeTop, lakeBottom uint32 = 4, 11

	// Frost is the witch's own way of moving: snow and ice price it low.
	Frost = cell.Domain(1 << 3)
)

// =========================== Game ===========================

// Demo is the effect demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — an ice witch writes the terrain",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// unit is the row every kind spawns from: where it starts and, if ordered, where it heads.
type unitRow struct {
	start, target cell.ID
	ordered       bool
}

type mainStage struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player
	shortcuts *players.Shortcuts

	witchy, mortal      *rule.Part
	witch, walker, boat kind.Of[unitRow]
	stack               game.Scenes
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "effect-demo" }

func (s *mainStage) Stack() game.Scenes { return s.stack }

// Init defines the game a section at a time, each building on those before it: the plugins, the
// player, the cells' kinds, the effects, the roles with their rules, the units' kinds, the scenes.
// How things look is the scene's (Layers).
func (s *mainStage) Init(ctx game.Initializer) error {
	if err := s.usePlugins(ctx); err != nil {
		return err
	}
	if err := s.definePlayer(); err != nil {
		return err
	}
	s.defineCells()
	s.defineEffects()
	s.defineRoles()
	s.defineKinds()
	return s.defineScenes(ctx)
}

func (s *mainStage) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
	})
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.selection = selection.NewPlugin(s.world)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	s.players = players.NewPlugin(s.world, s.selection, s.nav)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *mainStage) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *mainStage) defineCells() {
	s.board.CellKinds().Create(
		cell.Kind{Name: cell.Named("grass"), Cost: 2, Allows: cell.Land},
		cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land},
		cell.Kind{Name: cell.Named("water"), Cost: 1, Allows: cell.Water},
		cell.Kind{Name: cell.Named("snow"), Cost: 3, Allows: cell.Land | Frost}.Costing(Frost, 0.5),
		cell.Kind{Name: cell.Named("ice"), Cost: 2, Allows: cell.Land | Frost}.Costing(Frost, 0.5),
	)
}

func (s *mainStage) defineEffects() {
	snow, _ := s.board.CellKinds().Get("snow")
	ice, _ := s.board.CellKinds().Get("ice")
	effects := s.world.Effects()
	effects.Define("frost", effect.Spec{
		effect.Lasts(5 * time.Second),
		effect.Alter(func(g *cell.Ground) {
			switch g.Kind.Name.String() {
			// TODO: to tez nie jest do konca madre, bo nie powinien zmieniac sie typ, tego pola, ale podobnie jak dla encji powinnismy miec mapę rendition tych typow, czyli np. osniezona droga
			case "grass", "road":
				g.Kind = snow
			case "water":
				g.Kind = ice
			}
		}),
	})
	effects.Define("frozen", effect.Spec{
		effect.Alter(func(p *collision.Physics) { p.Mass = math.Inf(1) }),
		effect.Alter(func(st *steering.Steering) { st.Halted = true }),
	})
	effects.Define("slip", effect.Spec{
		effect.Alter(func(st *steering.Steering) { st.Brake = st.Accel / 8 }),
	})
}

// defineRoles says who does what: a kind playing a role obeys its rules, hooked with the kind.
func (s *mainStage) defineRoles() {
	ice, _ := s.board.CellKinds().Get("ice")
	effects := s.world.Effects()
	frost, frozen, slip := effects.Named("frost"), effects.Named("frozen"), effects.Named("slip")

	s.witchy = rule.Role("witch").Obeys(
		rule.Then[unit.Standing]("freeze", rule.All, rule.Around(1, rule.Apply(frost))),
	)
	s.mortal = rule.Role("mortal").Obeys(
		rule.Then[unit.Standing]("fallen in", rule.All,
			rule.If(unit.Standing.Fallen, rule.OneOf(
				rule.If(unit.On(ice), rule.Keep(frozen)),
				rule.Order(world.Despawn{}),
			))),
		rule.Then[unit.Standing]("on the ice", rule.All,
			rule.If(rule.Not(unit.Standing.Fallen), rule.If(unit.On(ice), rule.Keep(slip)))),
	)
}

func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	profile := func(brake float64) steering.Steering {
		return steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: brake, V0: UnitSpeed / 2, TurnRate: 0.15}
	}
	sel := comp.Tagged(s.selection.Tags().Selectable)
	mine := comp.Tagged(s.player.Owner())
	order := comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	s.witch = units.Define("witch", unit.Mover{Domain: cell.Land | cell.Water | Frost}, profile(UnitSpeed*4), sel, mine, order, rule.Plays(s.witchy, s.mortal))
	s.walker = units.Define("walker", unit.Mover{Domain: cell.Land}, profile(UnitSpeed*4), sel, mine, rule.Plays(s.mortal))
	s.boat = units.Define("boat", unit.Mover{Domain: cell.Water}, profile(UnitSpeed/4), sel, mine, order, rule.Plays(s.mortal))
}

func (s *mainStage) defineScenes(ctx game.Initializer) error {
	main := &mainScene{stage: s}
	main.keys = players.SceneKeys{
		{Key: control.KeyK, Label: "Shortcuts; Esc closes them", Do: func(rt game.Runtime, c game.Composition) { s.shortcuts.Open(rt, c) }},  // TODO: to powinien byc domyslny shortcut dostarczany przez worl plugin
		{Key: control.KeyEscape, Shift: true, Label: "Quit", Do: func(rt game.Runtime, _ game.Composition) { rt.Quit() }},                    // TODO: to powinien byc domyslny shortcut dostarczany przez worl plugin
		{Key: control.KeyB, Label: "Toggle the grid", Do: func(game.Runtime, game.Composition) { s.board.Res.Render.ToggleShowGridLines() }}, // TODO: to powinien byc domyslny shortcut dostarczany przez board plugin
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

func (s *mainStage) Spawn() error {
	brd := s.board.Res.Logic.Board
	cellAt := func(x, y uint32) cell.ID { c, _ := brd.CellIndex(x, y); return c }
	var cells []cell.Entry
	for y := lakeTop; y <= lakeBottom; y++ {
		for x := lakeLeft; x <= lakeRight; x++ {
			cells = append(cells, cell.Entry{Kind: "water", Cell: cellAt(x, y)})
		}
	}
	for x := uint32(1); x < GridWidth-1; x++ {
		cells = append(cells, cell.Entry{Kind: "road", Cell: cellAt(x, 1)}, cell.Entry{Kind: "road", Cell: cellAt(x, GridHeight-2)})
	}
	s.board.Seed(board.Layout{Default: "grass", Cells: cells})

	s.world.Seed(
		s.witch.Entry(unitRow{start: cellAt(2, 8), target: cellAt(GridWidth-3, 8)}),
		s.walker.Entry(unitRow{start: cellAt(2, 10)}),
		s.boat.Entry(unitRow{start: cellAt(lakeRight, 8), target: cellAt(lakeLeft, 8)}), // head-on into the witch's trail
	)
	return nil
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
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage

	// the units: each kind's own sprite, then its look under an effect, a row a look
	worldAtlas := render.NewAtlas()
	worldAtlas.RegisterAt(s.witch.SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 200, G: 230, B: 255, A: 255}))
	worldAtlas.RegisterAt(s.walker.SpriteID(), EntitySize, render.Solid(color.RGBA{R: 220, G: 90, B: 90, A: 255}))
	worldAtlas.RegisterAt(s.boat.SpriteID(), EntitySize, render.Solid(color.RGBA{R: 140, G: 90, B: 40, A: 255}))
	effects := s.world.Effects()
	for _, l := range []struct {
		effect string
		of     render.SpriteID
		draw   render.SpriteDrawer
	}{
		{"frozen", s.witch.SpriteID(), render.Diamond(color.RGBA{R: 240, G: 248, B: 255, A: 255})}, // the witch gone white
		{"frozen", s.walker.SpriteID(), render.Solid(color.RGBA{R: 235, G: 175, B: 175, A: 255})},  // the walker rimed
		{"frozen", s.boat.SpriteID(), func(dst *render.Canvas, size int) { // the boat in a rim of ice
			render.Solid(color.RGBA{R: 140, G: 90, B: 40, A: 255})(dst, size)
			render.Border(color.RGBA{R: 190, G: 220, B: 245, A: 255})(dst, size)
		}},
	} {
		worldAtlas.RegisterAt(effects.Named(l.effect).Look(l.of), EntitySize, l.draw)
	}
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKinds()
	boardAtlas := render.NewAtlas()
	for name, c := range map[string]color.RGBA{
		"grass": {R: 60, G: 95, B: 60, A: 255},
		"road":  {R: 150, G: 130, B: 80, A: 255},
		"water": {R: 40, G: 90, B: 170, A: 255},
		"snow":  {R: 235, G: 240, B: 245, A: 255},
		"ice":   {R: 170, G: 215, B: 240, A: 255},
	} {
		k, _ := kinds.Get(name)
		boardAtlas.RegisterAt(k.SpriteID, CellSize, render.Solid(c))
	}
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return []render.Layer{render.NewComposer(s.board.Renderer(), s.world.Renderer(), s.selection.Renderer(), s.nav.Renderer())}
}

func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.stage.players.EventHandler().HandleEvents(events)
	m.keys.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }
