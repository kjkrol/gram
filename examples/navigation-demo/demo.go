package main

import (
	"image/color"
	"log"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/rule"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
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
	MaxEntCount  = 16 // the units; the walls are cells, not entities

	saveBasePath = "board-navigation-demo"
)

type State struct{ Saves int }

// =========================== Game ===========================

// Demo is the board/navigation/selection demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram board & navigation plugins demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// mainStage wires the board/navigation/selection demo; its plugins are its own fields.
type mainStage struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player // the one at this keyboard: the units are its
	shortcuts *players.Shortcuts
	red, blue kind.Of[unit]
	// under is the cell each unit stood on last tick — where H opens a trapdoor.
	under map[uid.UID64]board.CellID
	stack game.Scenes
	state *State
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "board-navigation-demo" }

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
	s.registerCellKinds()
	s.under = map[uid.UID64]board.CellID{}
	if err := s.board.Hook(rule.On("standing", rule.All, func(m *rule.Moment[board.Standing]) rule.Step {
		return m.OneOf(
			m.If(board.Standing.Fallen, m.Steps(m.Call(s.fell), m.Order(world.Despawn{}))),
			m.Call(s.stands),
		)
	})); err != nil {
		return err
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

	s.state = &State{}

	s.defineKinds()

	main := &mainScene{stage: s}
	// the scene's own keys, labelled for the shortcuts list: K opens it, Esc closes it
	main.keys = players.SceneKeys{
		{Key: control.KeyK, Label: "Shortcuts; Esc closes them", Do: func(rt game.Runtime, c game.Composition) { s.shortcuts.Open(rt, c) }},
		{Key: control.KeyEscape, Shift: true, Label: "Quit", Do: func(rt game.Runtime, _ game.Composition) { rt.Quit() }},
		{Key: control.KeyB, Label: "Toggle the grid", Do: func(game.Runtime, game.Composition) { s.board.Res.Render.ToggleShowGridLines() }},
		{Key: control.KeyR, Label: "Build a road through the wall", Do: func(game.Runtime, game.Composition) {
			buildShortcut(s.board.Res.Logic.Board, s.board.CellKindDict())
			log.Print("built a road through the wall — in-flight units re-path onto it as soon as they deviate")
		}},
		{Key: control.KeyH, Label: "Open the trapdoors", Do: func(game.Runtime, game.Composition) { s.openTrapdoors() }},
		{Key: control.KeyF5, Label: "Save the game", Do: func(rt game.Runtime, _ game.Composition) {
			s.state.Saves++
			if err := rt.Persistence().Save(saveBasePath, "", s.state); err != nil {
				log.Printf("save: %v", err)
				return
			}
			log.Printf("saved (save #%d)", s.state.Saves)
		}},
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

// registerCellKinds defines every terrain kind the board can hold.
func (s *mainStage) registerCellKinds() {
	s.board.CellKindDict().Create(
		board.CellKind{Name: board.Named("grass"), Cost: 2, Allows: board.Land},
		board.CellKind{Name: board.Named("wall"), Cost: 1, Solid: true},
		board.CellKind{Name: board.Named("road"), Cost: 1, Allows: board.Land},
		board.CellKind{Name: board.Named("hole"), Cost: 1}, // admits nobody and is not solid: whoever stands on it falls
	)
}

// stands remembers where each unit stands.
func (s *mainStage) stands(_ plugin.Tick, st board.Standing) { s.under[st.ID] = st.Cell }

// fell forgets a unit that fell into a hole, as it gives itself a Despawn.
func (s *mainStage) fell(_ plugin.Tick, st board.Standing) {
	log.Printf("unit %d fell into the %s at cell %d", st.ID, st.Kind.Name, st.Cell)
	delete(s.under, st.ID)
}

// openTrapdoors turns the cell under every unit into a hole.
func (s *mainStage) openTrapdoors() {
	hole, _ := s.board.CellKindDict().Get("hole")
	for _, c := range s.under {
		s.board.Res.Logic.Board.Set(c, hole)
	}
	log.Printf("opened a hole under %d units", len(s.under))
}

func (s *mainStage) Restore(p game.Persistence) (bool, error) {
	saves, err := p.List(saveBasePath)
	if err != nil {
		return false, err
	}
	if !slices.Contains(saves, "") {
		return false, nil
	}
	if err := p.Load(saveBasePath, "", s.state); err != nil {
		return false, err
	}
	log.Printf("loaded saved board (save #%d)", s.state.Saves)
	return true, nil
}

// unit is the row the "red"/"blue" kinds spawn from: where the unit starts and where it heads.
type unit struct{ start, target board.CellID }

// defineKinds says what this game's entities are, fresh or restored.
func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unit](s.board, board.Shape{Size: EntitySize}, func(u unit) geom.Vec { return brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15}
	own := []comp.Comp{
		comp.Load(func(u unit) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} }),
		comp.Tagged(s.selection.Tags().Selectable, s.selection.Tags().Selected), comp.Tagged(s.player.Owner()),
	}
	s.red = units.Define("red", board.Mover{Domain: board.Land}, profile, own...)
	s.blue = units.Define("blue", board.Mover{Domain: board.Land}, profile, own...)
}

// Spawn says who is there when the game starts fresh.
func (s *mainStage) Spawn() error {
	brd := s.board.Res.Logic.Board
	cell := func(x, y uint32) board.CellID { c, _ := brd.CellIndex(x, y); return c }

	// A wall down column 12 from row 2, a road round it along row 1 and down both flanks, and a
	// hole on each unit's straight line, so the planner has to go round.
	var cells []board.CellEntry
	for y := uint32(2); y < GridHeight; y++ {
		cells = append(cells, board.CellEntry{Kind: "wall", Cell: cell(wallCol, y)})
	}
	cells = append(cells, board.CellEntry{Kind: "hole", Cell: cell(6, 4)}, board.CellEntry{Kind: "hole", Cell: cell(17, 12)})
	for x := roadLeft; x <= roadRight; x++ {
		cells = append(cells, board.CellEntry{Kind: "road", Cell: cell(x, roadTop)})
	}
	for y := roadTop + 1; y <= roadBottom; y++ {
		cells = append(cells, board.CellEntry{Kind: "road", Cell: cell(roadLeft, y)}, board.CellEntry{Kind: "road", Cell: cell(roadRight, y)})
	}
	s.board.Seed(board.Layout{Default: "grass", Cells: cells})

	s.world.Seed(
		s.red.Entry(unit{start: cell(2, 4), target: cell(GridWidth-3, 4)}),
		s.blue.Entry(unit{start: cell(2, 12), target: cell(GridWidth-3, 12)}),
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

	worldAtlas := render.NewAtlas()
	worldAtlas.RegisterAt(s.red.SpriteID(), EntitySize, render.Solid(color.RGBA{R: 220, G: 90, B: 90, A: 255}))
	worldAtlas.RegisterAt(s.blue.SpriteID(), EntitySize, render.Solid(color.RGBA{R: 90, G: 140, B: 220, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKindDict()
	grass, _ := kinds.Get("grass")
	wall, _ := kinds.Get("wall")
	road, _ := kinds.Get("road")
	hole, _ := kinds.Get("hole")
	boardAtlas := render.NewAtlas()
	boardAtlas.RegisterAt(grass.SpriteID, CellSize, render.Solid(color.RGBA{R: 60, G: 95, B: 60, A: 255}))
	boardAtlas.RegisterAt(wall.SpriteID, CellSize, render.Solid(color.RGBA{R: 40, G: 40, B: 40, A: 255}))
	boardAtlas.RegisterAt(road.SpriteID, CellSize, render.Solid(color.RGBA{R: 150, G: 130, B: 80, A: 255}))
	boardAtlas.RegisterAt(hole.SpriteID, CellSize, render.Solid(color.RGBA{R: 10, G: 10, B: 30, A: 255}))
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

const (
	wallCol     = 12
	shortcutRow = 8

	roadLeft, roadRight uint32 = 2, GridWidth - 3
	roadTop, roadBottom uint32 = 1, 13
)

// buildShortcut lays a road along shortcutRow from flank to flank, through the wall.
func buildShortcut(brd *board.Board, kinds board.CellKindDict) {
	road, _ := kinds.Get("road")
	for x := roadLeft + 1; x < roadRight; x++ {
		c, _ := brd.CellIndex(x, shortcutRow)
		brd.Set(c, road)
	}
}
