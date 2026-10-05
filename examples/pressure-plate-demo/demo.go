// Command pressure-plate-demo is two pressure plates and two strips of trapdoors across a meadow:
// wanderers nobody owns walk to and fro over both strips, and the player walks a scout onto a
// plate — while someone stands on it, and a second after, its trapdoors are open and every
// unfortunate on one falls in. A plate is a cell with a name, playing the role plate: stood on, it
// Triggers, and the command that names it opens the group of cells that is its strip (rule.Cast,
// entity.Named, entity.Group). All of it is defined here, in the game, and the rules hooked with
// ctx.Hook.
package main

import (
	"image/color"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
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
	EntitySize   = 20
	UnitSpeed    = CellSize * 2
	MaxEntCount  = 16 // the units; cell entities do not count

	// the strips of trapdoors run from top to bottom of the meadow, two cells wide; the plates and
	// the scouts are on the row under them
	stripTop, stripBottom uint32 = 2, 13
	plateRow                     = GridHeight - 2

	// heldAfter is how long a plate stays pressed once nobody stands on it.
	heldAfter = time.Second
)

// rows are where the wanderers walk to and fro, each across both strips.
var rows = []uint32{3, 6, 9, 12}

// groups are a plate each and the trapdoors it opens: the plate's column and the first of its
// strip's.
var groups = []struct {
	name        string
	plate, left uint32
}{{"west", 3, 7}, {"east", GridWidth - 4, 15}}

// =========================== Game ===========================

// Demo is the pressure plate demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — pressure plates and trapdoors",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// unit is the row every kind spawns from: where it starts and, for a wanderer, the other end of
// its walk.
type unitRow struct{ start, to cell.ID }

type mainStage struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player // the one at this keyboard: the scouts are its
	shortcuts *players.Shortcuts
	brd       *board.Board

	plate *rule.Part // a plate stood on sets off the command that names it

	scout    kind.Of[unitRow]
	wanderer kind.Of[unitRow]
	stack    game.Scenes
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "pressure-plate-demo" }

func (s *mainStage) Stack() game.Scenes { return s.stack }

func (s *mainStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
	})

	s.collision = collision.NewPlugin(s.world)
	if err := ctx.Use(s.collision); err != nil {
		return err
	}

	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.brd = s.board.Res.Logic.Board
	s.board.CellKinds().Create(
		cell.Kind{Name: cell.Named("grass"), Cost: 1, Allows: cell.Land},
		cell.Kind{Name: cell.Named("boards"), Cost: 1, Allows: cell.Land}, // a trapdoor shut
		cell.Kind{Name: cell.Named("plate"), Cost: 1, Allows: cell.Land},
		cell.Kind{Name: cell.Named("pit"), Cost: 1}, // holds nobody
	)
	pit, _ := s.board.CellKinds().Get("pit")

	// A trapdoor open is a pit, for a while after it was last opened.
	fx := s.world.Effects()
	open := fx.Define("open", effect.Spec{effect.Lasts(heldAfter), effect.Alter(func(g *cell.Ground) { g.Kind = pit })})

	// The game's rules, hooked once the plugins are in: whoever stands where nothing holds it
	// falls in, and a plate stood on sets off its command.
	s.plate = rule.Role("plate").Obeys(
		rule.Then[cell.Now]("press", rule.All, rule.If(cell.Now.Stood, rule.Trigger())))
	rules := []rule.Rule{s.plate, rule.Then[unit.Standing]("fall in", rule.All,
		rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{})))}
	// The commands: each plate opens its own strip of trapdoors, a group of cells.
	var commands []rule.Casting
	for _, g := range groups {
		commands = append(commands, rule.Cast(open).On(entity.Group("trapdoors "+g.name)).By(entity.Named("plate "+g.name)))
	}
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
	// The game's rules, each on the plugin in use that hosts its moment: here the board.
	if err := ctx.Hook(rules...); err != nil {
		return err
	}
	if err := ctx.Commands(commands...); err != nil {
		return err
	}
	s.defineKinds()

	main := &mainScene{stage: s}
	// the scene's own keys, labelled for the shortcuts list: K opens it, Esc closes it
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

// defineKinds says what this game's entities are: the player's scouts, and the wanderers,
// nobody's, each walking its row from one side of the meadow to the other and back, over both
// strips.
func (s *mainStage) defineKinds() {
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return s.brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, V0: UnitSpeed / 2, TurnRate: 0.15}
	land := unit.Mover{Domain: cell.Land}
	s.scout = units.Define("scout", land, profile)
	// a wanderer walks to the other end of its row and back, a second's rest at each end
	s.wanderer = units.Define("wanderer", land, profile,
		comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.Patrol(time.Second, u.to, u.start) }))
}

// Spawn lays the plates, each by its name, and the strips of trapdoors, each a group, and puts
// the scouts and the wanderers in place.
func (s *mainStage) Spawn() error {
	cellAt := func(x, y uint32) cell.ID { c, _ := s.brd.CellIndex(x, y); return c }
	var cells []cell.Entry
	for _, g := range groups {
		cells = append(cells, cell.Entry{Kind: "plate", Cell: cellAt(g.plate, plateRow), Roles: []*rule.Part{s.plate}, Name: "plate " + g.name})
		for y := stripTop; y <= stripBottom; y++ {
			for x := g.left; x <= g.left+1; x++ {
				cells = append(cells, cell.Entry{Kind: "boards", Cell: cellAt(x, y), Group: "trapdoors " + g.name})
			}
		}
	}
	s.board.Seed(board.Layout{Default: "grass", Cells: cells})

	for i := range uint32(3) {
		s.world.Seed(s.scout.Entry(unitRow{start: cellAt(GridWidth/2-2+2*i, plateRow)}).Told(players.Give{To: s.player.ID}, selection.Allow{}))
	}
	for _, row := range rows {
		s.world.Seed(s.wanderer.Entry(unitRow{start: cellAt(2, row), to: cellAt(GridWidth-3, row)}))
	}
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

	worldAtlas := render.NewAtlas()
	worldAtlas.RegisterAt(s.scout.SpriteID(), EntitySize, render.Solid(color.RGBA{R: 90, G: 140, B: 230, A: 255}))
	worldAtlas.RegisterAt(s.wanderer.SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 220, G: 150, B: 60, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKinds()
	boardAtlas := render.NewAtlas()
	for name, c := range map[string]color.RGBA{
		"grass":  {R: 60, G: 95, B: 60, A: 255},
		"boards": {R: 120, G: 90, B: 55, A: 255},
		"plate":  {R: 160, G: 160, B: 170, A: 255},
		"pit":    {R: 15, G: 12, B: 20, A: 255},
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

// Viewports are where the world is shown: the local players' views.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.stage.players.EventHandler().HandleEvents(events)
	m.keys.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }
