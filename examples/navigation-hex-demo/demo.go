package main

import (
	"image/color"
	"log"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
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

// The board is a parallelogram of pointy-top hexes in axial (q, r) coordinates: every row
// shifts right by half a hex, so the world is as wide as the last row reaches and the triangles
// either side hold no cells.
const (
	TPS        = 60
	GridWidth  = 20
	GridHeight = 12
	HexSize    = 24 // circumradius
	EntitySize = 22
	UnitSpeed  = HexSize * 3
	// MaxEntCount is the units; the walls are cells, not entities.
	MaxEntCount = 16

	saveBasePath = "board-navigation-hex-demo"
)

var (
	ScreenWidth  = int(math.Ceil(HexSize*(math.Sqrt(3)*(GridWidth-1)+math.Sqrt(3)/2*(GridHeight-1)) + 2*HexSize))
	ScreenHeight = int(math.Ceil(HexSize * (1.5*(GridHeight-1) + 2)))
)

type State struct{ Saves int }

// =========================== Game ===========================

// Demo is the navigation demo on a hex board — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: &mainStage{}} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram navigation on a hex board",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

// mainStage wires the hex board, navigation and selection; its plugins are its own fields.
type mainStage struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	player    *players.Player // the one at this keyboard: the units are its
	shortcuts *players.Shortcuts
	red, blue kind.Of[unitRow]
	stack     game.Scenes
	state     *State
}

var _ game.Stage = (*mainStage)(nil)

func (s *mainStage) Name() string { return "board-navigation-hex-demo" }

func (s *mainStage) Stack() game.Scenes { return s.stack }

func (s *mainStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: uint32(ScreenWidth), Height: uint32(ScreenHeight)},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
	})

	s.collision = collision.NewPlugin(s.world)
	if err := ctx.Use(s.collision); err != nil {
		return err
	}

	grid := grid.DefaultGrids{}.Hex(GridWidth, GridHeight, HexSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.registerCellKinds()
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
	s.board.CellKinds().Create(
		cell.Kind{Name: cell.Named("grass"), Cost: 2, Allows: cell.Land},
		cell.Kind{Name: cell.Named("wall"), Cost: 1, Solid: true},
		cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land},
	)
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
type unitRow struct{ start, target cell.ID }

// defineKinds says what this game's entities are, fresh or restored.
func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15}
	own := []comp.Comp{
		comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} }),
	}
	s.red = units.Define("red", unit.Mover{Domain: cell.Land}, profile, own...)
	s.blue = units.Define("blue", unit.Mover{Domain: cell.Land}, profile, own...)
}

// Spawn says who is there when the game starts fresh.
func (s *mainStage) Spawn() error {
	brd := s.board.Res.Logic.Board
	cellAt := func(x, y uint32) cell.ID { c, _ := brd.CellIndex(x, y); return c }

	// A wall down the q = wallCol column from r = 1 to the bottom, and a road round it: along
	// r = 0 and down both flanks (which slant with the rows, as every hex column does).
	var cells []cell.Entry
	for r := uint32(1); r < GridHeight; r++ {
		cells = append(cells, cell.Entry{Kind: "wall", Cell: cellAt(wallCol, r)})
	}
	for q := roadLeft; q <= roadRight; q++ {
		cells = append(cells, cell.Entry{Kind: "road", Cell: cellAt(q, roadTop)})
	}
	for r := roadTop + 1; r <= roadBottom; r++ {
		cells = append(cells, cell.Entry{Kind: "road", Cell: cellAt(roadLeft, r)}, cell.Entry{Kind: "road", Cell: cellAt(roadRight, r)})
	}
	s.board.Seed(board.Layout{Default: "grass", Cells: cells})

	s.world.Seed(
		s.red.Entry(unitRow{start: cellAt(3, 3), target: cellAt(GridWidth-4, 3)}).Told(players.Give{To: s.player.ID}, selection.Allow{Selected: true}),
		s.blue.Entry(unitRow{start: cellAt(3, 9), target: cellAt(GridWidth-4, 9)}).Told(players.Give{To: s.player.ID}, selection.Allow{Selected: true}),
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
	worldAtlas.RegisterAt(s.red.SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 220, G: 90, B: 90, A: 255}))
	worldAtlas.RegisterAt(s.blue.SpriteID(), EntitySize, render.Diamond(color.RGBA{R: 90, G: 140, B: 220, A: 255}))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKinds()
	grass, _ := kinds.Get("grass")
	wall, _ := kinds.Get("wall")
	road, _ := kinds.Get("road")
	boardAtlas := render.NewAtlas()
	boardAtlas.RegisterAt(grass.SpriteID, hexSprite, render.Hexagon(color.RGBA{R: 60, G: 95, B: 60, A: 255}))
	boardAtlas.RegisterAt(wall.SpriteID, hexSprite, render.Hexagon(color.RGBA{R: 40, G: 40, B: 40, A: 255}))
	boardAtlas.RegisterAt(road.SpriteID, hexSprite, render.Hexagon(color.RGBA{R: 150, G: 130, B: 80, A: 255}))
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
	// hexSprite is the texture side for a hex sprite: the hex's height, so nothing is upscaled.
	hexSprite = 2 * HexSize

	wallCol     = 10
	shortcutRow = 6

	roadLeft, roadRight uint32 = 2, GridWidth - 3
	roadTop, roadBottom uint32 = 0, GridHeight - 1
)

// buildShortcut lays a road along shortcutRow from flank to flank, through the wall.
func buildShortcut(brd *board.Board, kinds cell.Kinds) {
	road, _ := kinds.Get("road")
	for q := roadLeft + 1; q < roadRight; q++ {
		c, _ := brd.CellIndex(q, shortcutRow)
		brd.Set(c, road)
	}
}
