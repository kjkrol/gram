package main

import (
	"image/color"
	"log"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
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

func NewDemo() *Demo { return &Demo{stage: newStage()} }

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
	game.Stage // defined a section at a time: newStage

	mortal *rule.Part // whoever plays it falls in where nothing holds it

	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player // the one at this keyboard: the units are its
	shortcuts *players.Shortcuts
	red, blue kind.Of[unitRow]
	state     *State
}

// newStage defines the game a section at a time, each building on those before it.
func newStage() *mainStage {
	s := &mainStage{}
	s.Stage = stage.New("board-navigation-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Cells(s.defineCells).
		Rules(s.defineRules).
		Kinds(s.defineKinds).
		Scenes(s.defineScenes).
		Restore(s.restore).
		Layout(s.layOut).
		Units(s.placeUnits).
		Update(s.update)
	return s
}

func (s *mainStage) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
	})
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.selection = selection.NewPlugin(s.world)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	s.players = players.NewPlugin(s.world, s.selection, s.nav)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.selection, s.nav, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	s.state = &State{}
	return nil
}

func (s *mainStage) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

// defineRules says the one role: a mortal standing where nothing holds it falls in.
func (s *mainStage) defineRules() {
	s.world.Roles().Define("mortal",
		rule.Then[unit.Standing]("fall in", rule.All, rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))
	s.mortal = s.world.Roles().Named("mortal")
}

func (s *mainStage) defineScenes() []game.Scene {
	main := &mainScene{stage: s}
	// the scene's own keys, labelled for the shortcuts list: K opens it, Esc closes it
	main.keys = players.SceneKeys{
		{Key: control.KeyK, Label: "Shortcuts; Esc closes them", Do: func(rt game.Runtime, c game.Composition) { s.shortcuts.Open(rt, c) }},
		{Key: control.KeyEscape, Shift: true, Label: "Quit", Do: func(rt game.Runtime, _ game.Composition) { rt.Quit() }},
		{Key: control.KeyB, Label: "Toggle the grid", Do: func(game.Runtime, game.Composition) { s.board.Res.Render.ToggleShowGridLines() }},
		{Key: control.KeyR, Label: "Build a road through the wall", Do: func(game.Runtime, game.Composition) {
			buildShortcut(s.board.Res.Logic.Board, s.board.CellKinds())
			log.Print("built a road through the wall — in-flight units re-path onto it as soon as they deviate")
		}},
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
	return []game.Scene{main, s.shortcuts}
}

// defineCells defines every terrain kind the board can hold.
func (s *mainStage) defineCells() {
	s.board.CellKinds().Create(
		cell.Kind{Name: cell.Named("grass"), Cost: 2, Allows: cell.Land},
		cell.Kind{Name: cell.Named("wall"), Cost: 1, Solid: true},
		cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land},
		cell.Kind{Name: cell.Named("hole"), Cost: 1}, // admits nobody and is not solid: whoever stands on it falls
	)
}

func (s *mainStage) restore(p game.Persistence) (bool, error) {
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
type unitRow struct{ start, target cell.ID }

// defineKinds says what this game's entities are, fresh or restored.
func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15}
	own := []comp.Comp{
		comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} }),
		rule.Plays(s.mortal),
	}
	units.Define("red", unit.Mover{Domain: cell.Land}, profile, own...)
	s.red = units.Named("red")
	units.Define("blue", unit.Mover{Domain: cell.Land}, profile, own...)
	s.blue = units.Named("blue")
}

// cellAt is the cell at column x, row y.
func (s *mainStage) cellAt(x, y uint32) cell.ID {
	c, _ := s.board.Res.Logic.Board.CellIndex(x, y)
	return c
}

// layOut is the board a fresh game starts on.
func (s *mainStage) layOut() {
	// A wall down column 12 from row 2, a road round it along row 1 and down both flanks, and a
	// hole on each unit's straight line, so the planner has to go round.
	var cells []cell.Entry
	for y := uint32(2); y < GridHeight; y++ {
		cells = append(cells, cell.Entry{Kind: "wall", Cell: s.cellAt(wallCol, y)})
	}
	cells = append(cells, cell.Entry{Kind: "hole", Cell: s.cellAt(6, 4)}, cell.Entry{Kind: "hole", Cell: s.cellAt(17, 12)})
	for x := roadLeft; x <= roadRight; x++ {
		cells = append(cells, cell.Entry{Kind: "road", Cell: s.cellAt(x, roadTop)})
	}
	for y := roadTop + 1; y <= roadBottom; y++ {
		cells = append(cells, cell.Entry{Kind: "road", Cell: s.cellAt(roadLeft, y)}, cell.Entry{Kind: "road", Cell: s.cellAt(roadRight, y)})
	}
	s.board.Seed(board.Layout{Default: "grass", Cells: cells})
}

// placeUnits puts the two units in place, the player's and selected from the start.
func (s *mainStage) placeUnits() {
	mine := []any{players.Give{To: s.player.ID}, selection.Allow{Selected: true}}
	s.world.Seed(
		s.red.Entry(unitRow{start: s.cellAt(2, 4), target: s.cellAt(GridWidth-3, 4)}).Told(mine...),
		s.blue.Entry(unitRow{start: s.cellAt(2, 12), target: s.cellAt(GridWidth-3, 12)}).Told(mine...),
	)
}

func (s *mainStage) update(ctx goke.RunCtx, d time.Duration) {
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

	kinds := s.board.CellKinds()
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
func buildShortcut(brd *board.Board, kinds cell.Kinds) {
	road, _ := kinds.Get("road")
	for x := roadLeft + 1; x < roadRight; x++ {
		c, _ := brd.CellIndex(x, shortcutRow)
		brd.Set(c, road)
	}
}
