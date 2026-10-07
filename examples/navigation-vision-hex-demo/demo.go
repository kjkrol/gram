// Command navigation-vision-hex-demo puts sight on units navigating a hex board: their cones stop
// at the wall and fade in the forest, read from the hex cells themselves; a hawk flies over
// both and sees through the forest. Shift+C shows the cones, Shift+P the routes.
package main

import (
	"image/color"
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
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
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
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

// The board is a parallelogram of pointy-top hexes in axial (q, r) coordinates — see
// navigation-hex-demo.
const (
	TPS        = 60
	GridWidth  = 20
	GridHeight = 12
	HexSize    = 24
	EntitySize = 22
	UnitSpeed  = HexSize * 3
	// MaxEntCount is the units; the wall and the forests are cells, not entities.
	MaxEntCount = 32

	hexSprite   = 2 * HexSize
	sightRadius = 200
	sightHalf   = math.Pi / 5
)

var (
	ScreenWidth  = int(math.Ceil(HexSize*(math.Sqrt(3)*(GridWidth-1)+math.Sqrt(3)/2*(GridHeight-1)) + 2*HexSize))
	ScreenHeight = int(math.Ceil(HexSize * (1.5*(GridHeight-1) + 2)))
)

// =========================== Game ===========================

// Demo is the hex board + navigation + vision demo — exactly one Stage (arena below).
type Demo struct {
	a     *arena
	stage game.Stage
}

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo {
	a, st := newArena()
	return &Demo{a: a, stage: st}
}

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram — sight across a hex board: walls cut, forests dim, a hawk flies over",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

// =========================== Stage ===========================

type arena struct {
	world      *world.Plugin
	board      *board.Plugin
	topography *topography.Plugin
	nav        *navigation.Plugin
	driving    *driving.Plugin
	collision  *collision.Plugin
	selection  *selection.Plugin
	players    *players.Plugin
	cameras    *cameras.Plugin
	player     *players.Player // the one at this keyboard: the units are its
	vision     *vision.Plugin
}

// newStage defines the game a section at a time, each building on those before it.
// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New("board-navigation-vision-hex-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Cells(s.defineCells).
		Kinds(s.defineKinds).
		Scenes(s.defineScenes).
		Layout(s.layOut).
		Units(s.placeUnits).
		Update(s.update)
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: uint32(ScreenWidth), Height: uint32(ScreenHeight)},
		Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
		Heights:  true, // heights: the hawk looks over the wall, the forest and the hill
	})
	grid := grid.DefaultGrids{}.Hex(GridWidth, GridHeight, HexSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.topography = topography.NewPlugin(s.world, s.board, topography.Config{Cell: HexSize}) // the hills in relief, seen from above
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).WithCollision(s.collision)
	s.vision = vision.NewPlugin(s.world).WithBoard(s.board).WithLog(log.Default()).
		WithViews(render.Show(s.selection.IsSelected)) // only the selected ones' cones
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving, s.topography, s.vision)
	for _, p := range []plugin.Plugin{s.collision, s.board, s.topography, s.selection, s.nav, s.driving, s.cameras, s.players, s.vision} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player", s.cameras.New(s.topography.Views(topography.FromAbove), camera.Config{}))
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1))
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true, Allows: cell.Air, Veil: 1, Height: 10})
	kinds.Define(ForestCell, cell.Kind{Cost: 3, Allows: cell.Land | cell.Air, Veil: 0.6, Height: 8}.Costing(cell.Air, 1))
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land | cell.Air})
	kinds.Define(HillCell, cell.Kind{Cost: 2, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1))
}

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	return []game.Scene{main}
}

// unit is the row every unit kind spawns from: where it starts and where it heads.
type unitRow struct{ start, target cell.ID }

var unitColors = []color.RGBA{
	{R: 220, G: 90, B: 90, A: 255},
	{R: 90, G: 140, B: 220, A: 255},
	{R: 230, G: 200, B: 80, A: 255},
}

var hawkColor = color.RGBA{R: 120, G: 130, B: 60, A: 255}

// The ground's colours, one a kind.
var (
	grassColor  = color.RGBA{R: 60, G: 95, B: 60, A: 255}
	wallColor   = color.RGBA{R: 40, G: 40, B: 40, A: 255}
	forestColor = color.RGBA{R: 25, G: 60, B: 30, A: 255}
	roadColor   = color.RGBA{R: 150, G: 130, B: 80, A: 255}
	hillColor   = color.RGBA{R: 110, G: 100, B: 70, A: 255}
)

func (s *arena) defineKinds() {
	brd := s.board.Res.Logic.Board
	// Every unit is 2 tall; the eye is a fact of the kind, the altitude the board's to write.
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize, Height: 2}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	order := comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} })
	sight := comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: sightRadius, Ahead: true})
	eye := func(height float64) comp.Comp { return comp.Const(world.Eye{Height: height, Angle: 2 * sightHalf}) }
	scout := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.15}
	for _, name := range scouts {
		units.Define(name, unit.Mover{Domain: cell.Land}, scout, order, sight, eye(1.5))
	}
	// The hawk flies 40 above the ground on the Air plane: walls and walkers pass under it, and its
	// eye looks over the wall, the forest and the hill that stop a walker's.
	flyer := steering.Steering{MaxSpeed: UnitSpeed * 1.5, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.1}
	units.Define(HawkKind, unit.Mover{Domain: cell.Air, Lift: 40}, flyer, order,
		sight, eye(1))
}

// cellAt is the cell at column x, row y.
func (s *arena) layOut() {
	brd := s.board.Res.Logic.Board
	// A wall down the q = wallCol column with a gap at r = gapRow, a forest either side of the
	// gap, and a road along r = 0 with both flanks.
	var cells []cell.Entry
	for r := uint32(1); r < GridHeight; r++ {
		if r == gapRow {
			continue
		}
		cells = append(cells, cell.Entry{Kind: WallCell, Cell: brd.CellIndex(wallCol, r)})
	}
	for _, f := range [][2]uint32{{5, 4}, {13, 8}} {
		for dr := uint32(0); dr < 3; dr++ {
			for dq := uint32(0); dq < 3; dq++ {
				cells = append(cells, cell.Entry{Kind: ForestCell, Cell: brd.CellIndex(f[0]+dq, f[1]+dr)})
			}
		}
	}
	// A hill in the first unit's way: its cone climbs the slope and stops, the hawk's passes over.
	for dr := uint32(2); dr <= 4; dr++ {
		for dq := uint32(8); dq <= 10; dq++ {
			if dq != wallCol {
				cells = append(cells, cell.Entry{Kind: HillCell, Cell: brd.CellIndex(dq, dr)})
			}
		}
	}
	for q := roadLeft; q <= roadRight; q++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.CellIndex(q, roadTop)})
	}
	for r := roadTop + 1; r <= roadBottom; r++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.CellIndex(roadLeft, r)}, cell.Entry{Kind: RoadCell, Cell: brd.CellIndex(roadRight, r)})
	}
	hills := map[cell.ID]bool{}
	for _, e := range cells {
		hills[e.Cell] = e.Kind == HillCell
	}
	heights := relief.MeanOfCells(s.board.Res.Logic.Board, func(c cell.ID) float64 {
		if hills[c] {
			return hillHeight
		}
		return 0
	})
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
	s.topography.Seed(heights)
}

func (s *arena) placeUnits() {
	scoutKind := func(i int) kind.Of[unitRow] { return kind.Named[unitRow](s.world.Kinds(), scouts[i]) }
	hawkKind := kind.Named[unitRow](s.world.Kinds(), HawkKind)
	brd := s.board.Res.Logic.Board
	player := players.Give{To: s.player.ID}
	s.world.Seed(
		scoutKind(0).Entry(unitRow{start: brd.CellIndex(3, 3), target: brd.CellIndex(GridWidth-4, 3)}).Told(player, selection.Allow{Selected: true}),
		scoutKind(1).Entry(unitRow{start: brd.CellIndex(3, 9), target: brd.CellIndex(GridWidth-4, 9)}).Told(player, selection.Allow{Selected: true}),
		scoutKind(2).Entry(unitRow{start: brd.CellIndex(GridWidth-4, gapRow), target: brd.CellIndex(3, gapRow)}).Told(player, selection.Allow{Selected: true}),
		// The hawk crosses the wall and the second forest head-on.
		hawkKind.Entry(unitRow{start: brd.CellIndex(1, 9), target: brd.CellIndex(GridWidth-2, 9)}).Told(player, selection.Allow{}),
	)
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.driving.RunPlan(ctx, d)
	s.vision.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.topography.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	arena *arena
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.arena
	scoutKind := func(i int) kind.Of[unitRow] { return kind.Named[unitRow](s.world.Kinds(), scouts[i]) }
	hawkKind := kind.Named[unitRow](s.world.Kinds(), HawkKind)

	worldAtlas := render.NewAtlas()
	for i := range scouts {
		worldAtlas.Add(scoutKind(i), EntitySize, render.Diamond(unitColors[i]))
	}
	worldAtlas.Add(hawkKind, EntitySize, render.Diamond(hawkColor))
	worldAtlas.Close()
	s.world.WithRenderer(worldAtlas)

	boardAtlas := s.board.NewAtlas(hexSprite)
	boardAtlas.Add(GrassCell, render.Hexagon(grassColor))
	boardAtlas.Add(WallCell, render.Hexagon(wallColor))
	boardAtlas.Add(ForestCell, render.Hexagon(forestColor))
	boardAtlas.Add(RoadCell, render.Hexagon(roadColor))
	boardAtlas.Add(HillCell, render.Hexagon(hillColor))
	boardAtlas.Close()
	s.board.WithRenderer(boardAtlas)

	s.nav.WithRenderer(nil)

	s.vision.WithRenderer(nil)
	s.selection.WithRenderer(nil)

	return []render.Layer{render.NewComposer(s.topography.Renderer(), s.board.Renderer(), s.world.Renderer(), s.vision.Renderer(), s.selection.Renderer(), s.nav.Renderer())}
}

// Viewports are where the world is shown: the local players' views.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.arena.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.arena.players.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }

const (
	wallCol = 10
	gapRow  = 6

	roadLeft, roadRight uint32 = 2, GridWidth - 3
	roadTop, roadBottom uint32 = 0, GridHeight - 1
)

// hillHeight is how high the hill stands over the grass.
const hillHeight = 12
