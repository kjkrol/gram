// Command navigation-vision-demo puts sight on navigated units in a world with heights: their cones stop
// at the wall, fade in the forest and climb the hill; a hawk 40 up looks over all three. Shift+C
// shows the cones, Shift+P the routes.
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
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/topography/relief"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
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
	EntitySize   = 22
	UnitSpeed    = CellSize * 2
	MaxEntCount  = 32 // the units; the wall and the forests are cells, not entities

	sightRadius = 200
	sightHalf   = math.Pi / 5
)

// =========================== Game ===========================

// Demo is the board + navigation + vision demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: newStage()} }

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — sight across a board: walls cut, forests dim, a hawk flies over",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

type mainStage struct {
	game.Stage // defined a section at a time: newStage

	world      *world.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	nav        *navigation.Plugin
	collision  *collision.Plugin
	selection  *selection.Plugin
	players    *players.Plugin
	player     *players.Player // the one at this keyboard: the units are its
	shortcuts  *players.Shortcuts
	vision     *vision.Plugin
	kinds      []kind.Of[unitRow]
	hawk       kind.Of[unitRow]
}

// newStage defines the game a section at a time, each building on those before it.
func newStage() *mainStage {
	s := &mainStage{}
	s.Stage = stage.New("board-navigation-vision-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Cells(s.defineCells).
		Kinds(s.defineKinds).
		Looks(s.defineLooks).
		Scenes(s.defineScenes).
		Layout(s.layOut).
		Units(s.placeUnits).
		Update(s.update)
	return s
}

func (s *mainStage) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
		Heights:  true, // heights: the hawk looks over the wall, the forest and the hill
	})
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.topography = topography.NewPlugin(s.world, s.board, topography.Config{Cell: CellSize}) // the hills in relief, seen from above
	s.selection = selection.NewPlugin(s.world)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection).WithCollision(s.collision)
	s.vision = vision.NewPlugin(s.world).WithBoard(s.board).WithLog(log.Default())
	s.players = players.NewPlugin(s.world, s.selection, s.nav, s.topography, s.vision)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.topography, s.selection, s.nav, s.players, s.vision} {
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
		cell.Kind{Name: cell.Named("grass"), Cost: 2, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1),
		cell.Kind{Name: cell.Named("wall"), Cost: 1, Solid: true, Allows: cell.Air, Veil: 1, Height: 10},
		cell.Kind{Name: cell.Named("forest"), Cost: 3, Allows: cell.Land | cell.Air, Veil: 0.6, Height: 8}.Costing(cell.Air, 1),
		cell.Kind{Name: cell.Named("road"), Cost: 1, Allows: cell.Land | cell.Air},
		cell.Kind{Name: cell.Named("hill"), Cost: 2, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1),
	)
}

// defineLooks has the views drawn be the selected units' alone.
func (s *mainStage) defineLooks() error { return s.vision.Draw(render.Show(s.selection.IsSelected)) }

func (s *mainStage) defineScenes() []game.Scene {
	main := &mainScene{stage: s}
	// the scene's own keys, labelled for the shortcuts list: K opens it, Esc closes it
	main.keys = players.SceneKeys{
		{Key: control.KeyK, Label: "Shortcuts; Esc closes them", Do: func(rt game.Runtime, c game.Composition) { s.shortcuts.Open(rt, c) }},
		{Key: control.KeyEscape, Shift: true, Label: "Quit", Do: func(rt game.Runtime, _ game.Composition) { rt.Quit() }},
		{Key: control.KeyB, Label: "Toggle the grid", Do: func(game.Runtime, game.Composition) { s.board.Res.Render.ToggleShowGridLines() }},
	}
	s.shortcuts = s.players.Shortcuts(main.keys)
	return []game.Scene{main, s.shortcuts}
}

// unit is the row every unit kind spawns from: where it starts and where it heads.
type unitRow struct{ start, target cell.ID }

var unitColors = []color.RGBA{
	{R: 220, G: 90, B: 90, A: 255},
	{R: 90, G: 140, B: 220, A: 255},
	{R: 230, G: 200, B: 80, A: 255},
}

var hawkColor = color.RGBA{R: 120, G: 130, B: 60, A: 255}

// defineKinds says what this game's entities are: one kind per colour, all scouts, and a hawk.
func (s *mainStage) defineKinds() {
	brd := s.board.Res.Logic.Board
	// Every unit is 2 tall; the eye is a fact of the kind, the altitude the board's to write.
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize, Height: 2}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	order := comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	sight := comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: sightRadius, Ahead: true})
	eye := func(height float64) comp.Comp { return comp.Const(world.Eye{Height: height, Angle: 2 * sightHalf}) }
	scout := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15}
	for _, name := range []string{"red", "blue", "yellow"} {
		units.Define(name, unit.Mover{Domain: cell.Land}, scout, order, sight, eye(1.5))
		s.kinds = append(s.kinds, units.Named(name))
	}
	// The hawk flies 40 above the ground on the Air plane: walls and walkers pass under it, and its
	// eye looks over the wall, the forest and the hill that stop a walker's.
	flyer := steering.Steering{MaxSpeed: UnitSpeed * 1.5, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.1}
	units.Define("hawk", unit.Mover{Domain: cell.Air, Lift: 40}, flyer, order,
		sight, eye(1))
	s.hawk = units.Named("hawk")
}

// cellAt is the cell at column x, row y.
func (s *mainStage) cellAt(x, y uint32) cell.ID {
	c, _ := s.board.Res.Logic.Board.CellIndex(x, y)
	return c
}

// layOut is the board a fresh game starts on, and the heights of its hills.
func (s *mainStage) layOut() {
	// A wall down column 12 with a gap at row 8, a forest either side of the gap, and a road
	// along row 1 with both flanks.
	var cells []cell.Entry
	for y := uint32(2); y < GridHeight; y++ {
		if y == gapRow {
			continue
		}
		cells = append(cells, cell.Entry{Kind: "wall", Cell: s.cellAt(wallCol, y)})
	}
	for _, f := range [][2]uint32{{6, 6}, {17, 10}} {
		for dy := uint32(0); dy < 3; dy++ {
			for dx := uint32(0); dx < 4; dx++ {
				cells = append(cells, cell.Entry{Kind: "forest", Cell: s.cellAt(f[0]+dx, f[1]+dy)})
			}
		}
	}
	// A hill in the first unit's way: its cone climbs the slope and stops, the hawk's passes over.
	for dy := uint32(3); dy <= 5; dy++ {
		for dx := uint32(7); dx <= 9; dx++ {
			cells = append(cells, cell.Entry{Kind: "hill", Cell: s.cellAt(dx, dy)})
		}
	}
	for x := roadLeft; x <= roadRight; x++ {
		cells = append(cells, cell.Entry{Kind: "road", Cell: s.cellAt(x, roadTop)})
	}
	for y := roadTop + 1; y <= roadBottom; y++ {
		cells = append(cells, cell.Entry{Kind: "road", Cell: s.cellAt(roadLeft, y)}, cell.Entry{Kind: "road", Cell: s.cellAt(roadRight, y)})
	}
	hills := map[cell.ID]bool{}
	for _, e := range cells {
		hills[e.Cell] = e.Kind == "hill"
	}
	heights := relief.MeanOfCells(s.board.Res.Logic.Board, func(c cell.ID) float64 {
		if hills[c] {
			return hillHeight
		}
		return 0
	})
	s.board.Seed(board.Layout{Default: "grass", Cells: cells})
	s.topography.Seed(heights)
}

// placeUnits puts the scouts, selected from the start, and the hawk in place, all the player's.
func (s *mainStage) placeUnits() {
	mine := players.Give{To: s.player.ID}
	s.world.Seed(
		s.kinds[0].Entry(unitRow{start: s.cellAt(2, 4), target: s.cellAt(GridWidth-3, 4)}).Told(mine, selection.Allow{Selected: true}),
		s.kinds[1].Entry(unitRow{start: s.cellAt(2, 12), target: s.cellAt(GridWidth-3, 12)}).Told(mine, selection.Allow{Selected: true}),
		s.kinds[2].Entry(unitRow{start: s.cellAt(GridWidth-3, gapRow), target: s.cellAt(2, gapRow)}).Told(mine, selection.Allow{Selected: true}),
		// The hawk crosses the wall and the second forest head-on.
		s.hawk.Entry(unitRow{start: s.cellAt(1, 11), target: s.cellAt(GridWidth-2, 11)}).Told(mine, selection.Allow{}),
	)
}

func (s *mainStage) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.vision.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.topography.RunPlan(ctx, d)
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
	for i, k := range s.kinds {
		worldAtlas.RegisterAt(k.SpriteID(), EntitySize, render.Solid(unitColors[i]))
	}
	worldAtlas.RegisterAt(s.hawk.SpriteID(), EntitySize, render.Diamond(hawkColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	kinds := s.board.CellKinds()
	boardAtlas := render.NewAtlas()
	for name, c := range map[string]color.RGBA{
		"grass":  {R: 60, G: 95, B: 60, A: 255},
		"wall":   {R: 40, G: 40, B: 40, A: 255},
		"forest": {R: 25, G: 60, B: 30, A: 255},
		"road":   {R: 150, G: 130, B: 80, A: 255},
		"hill":   {R: 110, G: 100, B: 70, A: 255},
	} {
		k, _ := kinds.Get(name)
		boardAtlas.RegisterAt(k.SpriteID, CellSize, render.Solid(c))
	}
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)

	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return []render.Layer{render.NewComposer(s.topography.Renderer(), s.board.Renderer(), s.world.Renderer(), s.vision.Renderer(), s.selection.Renderer(), s.nav.Renderer())}
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
	wallCol = 12
	gapRow  = 8

	roadLeft, roadRight uint32 = 2, GridWidth - 3
	roadTop, roadBottom uint32 = 1, 13
)

// hillHeight is how high the hill stands over the grass.
const hillHeight = 12
