// Command pressure-plate-demo is two pressure plates and two strips of trapdoors across a meadow:
// wanderers nobody owns walk to and fro over both strips, and the player walks a scout onto a
// plate — while someone stands on it, and a second after, its trapdoors are open and every
// unfortunate on one falls in. A plate pressed is a state of the whole game, put on the world
// (world.Apply) every step by whoever stands on the plate; a plate and a trapdoor are cells tagged
// with their group (board.Places). All of it is defined here, in the game.
package main

import (
	"image/color"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/board"
	bhooks "github.com/kjkrol/gram/plugins/board/hooks"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/rule"
	"github.com/kjkrol/gram/plugins/world/rule/effect"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
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
type unit struct{ start, to board.CellID }

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

	pressed           []effect.Effect // each group's plate, in the order of groups
	plates, trapdoors []tag.Tag[board.Places]

	scout    kind.Of[unit]
	wanderer kind.Of[unit]
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

	grid := board.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &board.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.brd = s.board.Res.Logic.Board
	s.board.CellKindDict().Create(
		board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land},
		board.CellKind{Name: board.Named("boards"), Cost: 1, Allows: board.Land}, // a trapdoor shut
		board.CellKind{Name: board.Named("plate"), Cost: 1, Allows: board.Land},
		board.CellKind{Name: board.Named("pit"), Cost: 1}, // holds nobody
	)
	pit, _ := s.board.CellKindDict().Get("pit")

	// A trapdoor open is a pit; a plate pressed, below, a state of the game for each group.
	fx := s.world.Effects()
	open := fx.Define("open", effect.Spec{effect.Alter(func(g *board.Ground) { g.Kind = pit })})

	// Whoever stands where nothing holds it falls in.
	if err := s.board.Hook(bhooks.LogFalls(), rule.On("fall in", rule.All, func(m *rule.Moment[board.Standing]) rule.Step {
		return m.If(board.Standing.Fallen, m.Order(world.Despawn{}))
	})); err != nil {
		return err
	}
	// Each group: the plate pressed, a state of the game; the tags of its plate and its trapdoors;
	// the rule of whoever stands on the plate pressing it, and the trapdoors' keeping them open
	// while it is pressed.
	for _, g := range groups {
		pressed := fx.Define("pressed "+g.name, effect.Spec{effect.Lasts(heldAfter)})
		plate := s.world.Kinds().DefineTag[board.Places]("plate " + g.name)
		trapdoor := s.world.Kinds().DefineTag[board.Places]("trapdoor " + g.name)
		if err := s.board.Hook(
			rule.On("plate "+g.name, rule.All, func(m *rule.Moment[board.Standing]) rule.Step {
				return m.If(func(st board.Standing) bool { return st.Places.Has(plate) }, m.Order(world.Apply{Effect: pressed}))
			}),
			rule.On("trapdoors "+g.name, rule.Self(trapdoor), func(m *rule.Moment[board.Cell]) rule.Step {
				return m.During(pressed, m.Keep(open))
			}),
		); err != nil {
			return err
		}
		s.pressed = append(s.pressed, pressed)
		s.plates, s.trapdoors = append(s.plates, plate), append(s.trapdoors, trapdoor)
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
	units := board.NewUnits[unit](s.board, board.Shape{Size: EntitySize}, func(u unit) geom.Vec { return s.brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, V0: UnitSpeed / 2, TurnRate: 0.15}
	land := board.Mover{Domain: board.Land}
	s.scout = units.Define("scout", land, profile, comp.Tagged(s.selection.Tags().Selectable), comp.Tagged(s.player.Owner()))
	// a wanderer walks to the other end of its row and back, a second's rest at each end
	s.wanderer = units.Define("wanderer", land, profile,
		comp.Load(func(u unit) navigation.MoveOrder { return navigation.Patrol(time.Second, u.to, u.start) }))
}

// Spawn lays the plates and the strips of trapdoors, each cell tagged with its group's, and puts
// the scouts and the wanderers in place.
func (s *mainStage) Spawn() error {
	cell := func(x, y uint32) board.CellID { c, _ := s.brd.CellIndex(x, y); return c }
	var cells []board.CellEntry
	for i, g := range groups {
		cells = append(cells, board.CellEntry{Kind: "plate", Cell: cell(g.plate, plateRow), Tags: tag.Tags[board.Places](0).With(s.plates[i])})
		trapdoors := tag.Tags[board.Places](0).With(s.trapdoors[i])
		for y := stripTop; y <= stripBottom; y++ {
			for x := g.left; x <= g.left+1; x++ {
				cells = append(cells, board.CellEntry{Kind: "boards", Cell: cell(x, y), Tags: trapdoors})
			}
		}
	}
	s.board.Seed(board.Layout{Default: "grass", Cells: cells})

	for i := range uint32(3) {
		s.world.Seed(s.scout.Entry(unit{start: cell(GridWidth/2-2+2*i, plateRow)}))
	}
	for _, row := range rows {
		s.world.Seed(s.wanderer.Entry(unit{start: cell(2, row), to: cell(GridWidth-3, row)}))
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

	kinds := s.board.CellKindDict()
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
