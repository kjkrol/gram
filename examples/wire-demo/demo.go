// Command wire-demo is three commands on one meadow, each a sentence saying what it does, whom
// it is for and who sets it off (rule.Cast, Toggle; entity.Named, entity.Group). "Open the west
// trapdoors" is the west lever's: 1 gives it, and so does a scout pulling the lever beside it (U).
// "Open the east trapdoors" is the pressure plate's in the yard, given while someone stands on it;
// the trapdoors stay open two seconds after. "Flip the gate" is the player's alone: G opens it and
// it stays open until G again. Two strips of trapdoors cross the meadow, each a group of cells, and
// a fence shuts the yard off from it, its gate a group too. A plate stood on and a lever pulled
// only Trigger: which command that sets off, the command says with By. Wanderers nobody owns walk
// to and fro over both strips; the player's scouts and porters start in the yard. Everyone plays
// mortal and falls in where nothing holds them; the scouts play hasty too, and J hastens the
// selected ones (selection.Selected), never the porters. All of it is defined here, in the game.
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
	MaxEntCount  = 16 // the units; cell entities and wires do not count

	// the strips of trapdoors run down the meadow, two cells wide from their left column
	stripTop, stripBottom uint32 = 1, 11
	westLeft, eastLeft    uint32 = 7, 15
	// the fence runs across under the meadow, its gate two cells from gateLeft; the yard under it
	// holds the plate, the scouts on its first row and the porters on its second
	fenceRow, gateLeft uint32 = 13, 11
	yardRow, plateCol  uint32 = 14, GridWidth - 4
	// the west lever stands at the yard's left end, on the scouts' row
	leverCol uint32 = 1

	// pulse is how long a wire stays on once its lever is pulled or its plate let go.
	pulse = 2 * time.Second
	// pulling is how long a scout pulls the lever beside it once told to.
	pulling = time.Second / 4
	// hasteHeld is how long the scouts go twice as fast.
	hasteHeld = 3 * time.Second
)

// rows are where the wanderers walk to and fro, each across both strips.
var rows = []uint32{2, 5, 8, 11}

// =========================== Game ===========================

// Demo is the wire demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — wires: a lever, a plate and a switch",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// unitRow is the row every kind spawns from: where it starts and, for a wanderer, the other end of
// its walk.
type unitRow struct{ start, to cell.ID }

type mainStage struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player // the one at this keyboard: the scouts and the porters are its
	shortcuts *players.Shortcuts
	brd       *board.Board

	roles       struct{ plate, lever, hasty, handy, mortal *rule.Part }
	commands    struct{ openWest, openEast, flipGate, hasten, reach rule.Casting }
	haste       effect.Effect
	hasteSprite render.SpriteID

	scout, porter, wanderer kind.Of[unitRow]
	stack                   game.Scenes
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "wire-demo" }

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
		cell.Kind{Name: cell.Named("pit"), Cost: 1},                       // holds nobody
		cell.Kind{Name: cell.Named("plate"), Cost: 1, Allows: cell.Land},
		cell.Kind{Name: cell.Named("lever"), Cost: 1, Allows: cell.Land},
		cell.Kind{Name: cell.Named("fence"), Cost: 1, Solid: true},
		cell.Kind{Name: cell.Named("gate"), Cost: 1, Solid: true},          // the gate shut
		cell.Kind{Name: cell.Named("gateway"), Cost: 1, Allows: cell.Land}, // the gate open
	)
	pit, _ := s.board.CellKinds().Get("pit")
	gateway, _ := s.board.CellKinds().Get("gateway")

	// The states, each an effect. On the cells: a trapdoor open, a pit, for a while; the gate open,
	// a gateway, until shut. On a scout: hastened, twice as fast and drawn bright; pulling a lever.
	fx := s.world.Effects()
	open := fx.Define("open", effect.Spec{effect.Lasts(pulse), effect.Alter(func(g *cell.Ground) { g.Kind = pit })})
	ajar := fx.Define("ajar", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind = gateway })})
	s.hasteSprite = s.world.Kinds().NewSprite()
	hasteSprite := s.hasteSprite
	s.haste = fx.Define("haste", effect.Spec{
		effect.Lasts(hasteHeld),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed, st.Accel = st.MaxSpeed*2, st.Accel*2 }),
		effect.Alter(func(a *world.Appearance) { a.SpriteID = hasteSprite }),
	})
	pull := fx.Define("pull", effect.Spec{effect.Lasts(pulling)})

	// The roles: what sets a command off, and what the units can do and suffer. A plate stood on
	// and a lever pulled Trigger; which command that is, the command says (By).
	s.roles.plate = rule.Role("plate").Obeys(
		rule.Then[cell.Now]("press", rule.All, rule.If(cell.Now.Stood, rule.Trigger())))
	s.roles.lever = rule.Role("lever") // does nothing of its own: a handy unit beside it pulls it
	s.roles.hasty = rule.Role("hasty")
	s.roles.handy = rule.Role("handy").Obeys(
		rule.Then[unit.Standing]("pull the lever beside", rule.All,
			rule.Under(pull, rule.Around(1, rule.Playing(s.roles.lever, rule.Trigger())))))
	s.roles.mortal = rule.Role("mortal").Obeys(
		rule.Then[unit.Standing]("fall in", rule.All, rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))

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

	// The commands: all that can be asked for in this game, each saying whom it is for and, where a
	// cell sets it off, which one.
	s.commands.openWest = rule.Cast(open).On(entity.Group("west trapdoors")).By(entity.Named("west lever"))
	s.commands.openEast = rule.Cast(open).On(entity.Group("east trapdoors")).By(entity.Named("plate"))
	s.commands.flipGate = rule.Toggle(ajar).On(entity.Group("gate"))
	s.commands.hasten = rule.Cast(s.haste).On(s.selection.Selected(s.roles.hasty))
	s.commands.reach = rule.Cast(pull).On(s.selection.Selected(s.roles.handy))
	if err := ctx.Commands(s.commands.openWest, s.commands.openEast, s.commands.flipGate); err != nil {
		return err
	}

	// The player's keys, each giving a command.
	if err := s.player.Bind(
		control.Give(control.KeyPress{Key: control.Key1}, "Pull the west lever: its trapdoors open for a while", s.commands.openWest),
		control.Give(control.KeyPress{Key: control.KeyG}, "Flip the gate's switch: open, or shut", s.commands.flipGate),
		control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts", s.commands.hasten),
		control.Give(control.KeyPress{Key: control.KeyU}, "Pull the lever beside the selected scouts", s.commands.reach),
	); err != nil {
		return err
	}

	// Every role's rules, each hooked on the plugin hosting its moment: the board, here.
	if err := ctx.Hook(s.roles.plate, s.roles.handy, s.roles.mortal); err != nil {
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

// defineKinds says what this game's entities are: the player's scouts, hasty and mortal, and its
// porters, laden and mortal alone; and the wanderers, nobody's and mortal, each walking its row
// from one side of the meadow to the other and back, over both strips.
func (s *mainStage) defineKinds() {
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return s.brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, V0: UnitSpeed / 2, TurnRate: 0.15}
	laden := steering.Steering{MaxSpeed: UnitSpeed * 3 / 4, Accel: UnitSpeed, V0: UnitSpeed / 4, TurnRate: 0.1}
	land := unit.Mover{Domain: cell.Land}
	s.scout = units.Define("scout", land, profile, rule.Plays(s.roles.hasty, s.roles.handy, s.roles.mortal))
	s.porter = units.Define("porter", land, laden, rule.Plays(s.roles.mortal))
	// a wanderer walks to the other end of its row and back, a second's rest at each end
	s.wanderer = units.Define("wanderer", land, profile, rule.Plays(s.roles.mortal),
		comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.Patrol(time.Second, u.to, u.start) }))
}

// Spawn lays the strips of trapdoors, each a group, the fence with the gate, the plate and the
// lever, each called what the commands call them, and puts the units in place.
func (s *mainStage) Spawn() error {
	cellAt := func(x, y uint32) cell.ID { c, _ := s.brd.CellIndex(x, y); return c }
	playing := func(r *rule.Part) []*rule.Part { return []*rule.Part{r} }
	var cells []cell.Entry
	strips := []struct {
		group string
		left  uint32
	}{{"west trapdoors", westLeft}, {"east trapdoors", eastLeft}}
	for _, strip := range strips {
		for y := stripTop; y <= stripBottom; y++ {
			for x := strip.left; x <= strip.left+1; x++ {
				cells = append(cells, cell.Entry{Kind: "boards", Cell: cellAt(x, y), Group: strip.group})
			}
		}
	}
	for x := range uint32(GridWidth) {
		if x == gateLeft || x == gateLeft+1 {
			cells = append(cells, cell.Entry{Kind: "gate", Cell: cellAt(x, fenceRow), Group: "gate"})
		} else {
			cells = append(cells, cell.Entry{Kind: "fence", Cell: cellAt(x, fenceRow)})
		}
	}
	cells = append(cells, cell.Entry{Kind: "plate", Cell: cellAt(plateCol, yardRow), Roles: playing(s.roles.plate), Name: "plate"})
	cells = append(cells, cell.Entry{Kind: "lever", Cell: cellAt(leverCol, yardRow), Roles: playing(s.roles.lever), Name: "west lever"})
	s.board.Seed(board.Layout{Default: "grass", Cells: cells})

	for i := range uint32(3) {
		s.world.Seed(s.scout.Entry(unitRow{start: cellAt(3+2*i, yardRow)}).Told(players.Give{To: s.player.ID}, selection.Allow{}))
	}
	for i := range uint32(2) {
		s.world.Seed(s.porter.Entry(unitRow{start: cellAt(4+2*i, yardRow+1)}).Told(players.Give{To: s.player.ID}, selection.Allow{}))
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
	worldAtlas.RegisterAt(s.hasteSprite, EntitySize, render.Solid(color.RGBA{R: 170, G: 220, B: 255, A: 255}))
	worldAtlas.RegisterAt(s.porter.SpriteID(), EntitySize, render.Solid(color.RGBA{R: 150, G: 110, B: 200, A: 255}))
	worldAtlas.RegisterAt(s.wanderer.SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 220, G: 150, B: 60, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKinds()
	boardAtlas := render.NewAtlas()
	for name, c := range map[string]color.RGBA{
		"grass":   {R: 60, G: 95, B: 60, A: 255},
		"boards":  {R: 120, G: 90, B: 55, A: 255},
		"pit":     {R: 15, G: 12, B: 20, A: 255},
		"plate":   {R: 160, G: 160, B: 170, A: 255},
		"lever":   {R: 200, G: 170, B: 60, A: 255},
		"fence":   {R: 85, G: 60, B: 40, A: 255},
		"gate":    {R: 70, G: 75, B: 90, A: 255},
		"gateway": {R: 150, G: 130, B: 95, A: 255},
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
