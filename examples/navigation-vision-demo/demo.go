package main

import (
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
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
	"github.com/kjkrol/gram/ui"
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
	MaxEntCount  = 32
)

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
	player     *players.Player
	vision     *vision.Plugin
}

func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram navigation vision demo",
		Description: "Sight across a board: walls cut, forests dim, a hawk flies over.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(BoardNavigationVisionStage).
			World(world.Config{
				Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
				Entities: world.EntitiesCfg{
					MaxCount: MaxEntCount,
					MinSize:  EntitySize,
					MaxSize:  EntitySize,
				},
				Heights: true,
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayer).
			Kinds(s.defineCells, s.defineKinds).
			Spawn(s.spawnCells, s.spawnUnits).
			Scenes(s.defineScenes).
			Update(s.update))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.topography = topography.NewPlugin(s.world, s.board, topography.Config{Cell: CellSize})
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision)
	s.vision = vision.NewPlugin(s.world).WithBoard(s.board).WithLog(log.Default())
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(
		s.world,
		s.cameras,
		s.board,
		s.selection,
		s.nav,
		s.driving,
		s.topography,
		s.vision,
	)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1))
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true, Allows: cell.Air, Veil: 1, Height: 10})
	kinds.Define(
		ForestCell,
		cell.Kind{Cost: 3, Allows: cell.Land | cell.Air, Veil: 0.6, Height: 8}.Costing(cell.Air, 1),
	)
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land | cell.Air})
	kinds.Define(HillCell, cell.Kind{Cost: 2, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1))
}

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
}

type unitRow struct{ start, target cell.ID }

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[unitRow](
		s.board,
		board.Shape{Size: EntitySize, Height: 2},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.start) },
	)
	order := comp.Load(
		func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} },
	)
	sight := comp.Const(vision.Sight{Facing: geom.NewVec(1, 0), Radius: 200, Ahead: true})
	eye := func(height float64) comp.Comp {
		return comp.Const(world.Eye{Height: height, Angle: 2 * math.Pi / 5})
	}
	scout := steering.Steering{
		MaxSpeed: UnitSpeed,
		Accel:    UnitSpeed * 2,
		Brake:    UnitSpeed * 4,
		V0:       UnitSpeed / 2,
		TurnRate: 0.15,
	}
	for _, name := range scouts {
		units.Define(name, unit.Mover{Domain: cell.Land}, scout, order, sight, eye(1.5))
	}
	flyer := steering.Steering{
		MaxSpeed: UnitSpeed * 1.5,
		Accel:    UnitSpeed * 2,
		Brake:    UnitSpeed * 4,
		V0:       UnitSpeed / 2,
		TurnRate: 0.1,
	}
	units.Define(HawkKind, unit.Mover{Domain: cell.Air, Lift: 40}, flyer, order,
		sight, eye(1))
}

// TODO: definicja planszy i rozmieszczenie jednostek w z pliku
func (s *arena) spawnCells() {
	brd := s.board.Board()
	var cells []cell.Entry
	for y := uint32(2); y < GridHeight; y++ {
		if y == 8 {
			continue
		}
		cells = append(cells, cell.Entry{Kind: WallCell, Cell: brd.At(12, y)})
	}
	for _, f := range [][2]uint32{{6, 6}, {17, 10}} {
		for dy := uint32(0); dy < 3; dy++ {
			for dx := uint32(0); dx < 4; dx++ {
				cells = append(cells, cell.Entry{Kind: ForestCell, Cell: brd.At(f[0]+dx, f[1]+dy)})
			}
		}
	}
	// A hill in the first unit's way: its cone climbs the slope and stops, the hawk's passes over.
	for dy := uint32(3); dy <= 5; dy++ {
		for dx := uint32(7); dx <= 9; dx++ {
			cells = append(cells, cell.Entry{Kind: HillCell, Cell: brd.At(dx, dy)})
		}
	}
	for x := uint32(2); x <= GridWidth-3; x++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.At(x, 1)})
	}
	for y := uint32(2); y <= 13; y++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.At(2, y)})
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.At(GridWidth-3, y)})
	}
	hills := map[cell.ID]bool{}
	for _, e := range cells {
		hills[e.Cell] = e.Kind == HillCell
	}
	heights := relief.MeanOfCells(s.board.Board(), func(c cell.ID) float64 {
		if hills[c] {
			return 12
		}
		return 0
	})
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
	s.topography.Seed(heights)
}

func (s *arena) spawnUnits() {
	scoutKind := func(i int) kind.Of[unitRow] {
		return kind.Named[unitRow](s.world.Kinds(), scouts[i])
	}
	hawkKind := kind.Named[unitRow](s.world.Kinds(), HawkKind)
	brd := s.board.Board()
	red := scoutKind(0).Entry(unitRow{start: brd.At(2, 4), target: brd.At(GridWidth-3, 4)})
	blue := scoutKind(1).Entry(unitRow{start: brd.At(2, 12), target: brd.At(GridWidth-3, 12)})
	yellow := scoutKind(2).Entry(unitRow{start: brd.At(GridWidth-3, 8), target: brd.At(2, 8)})
	// The hawk crosses the wall and the second forest head-on.
	hawk := hawkKind.Entry(unitRow{start: brd.At(1, 11), target: brd.At(GridWidth-2, 11)})
	player := players.Give{To: s.player.ID}
	selected := selection.Allow{Selected: true}
	s.world.Seed(
		red.Told(player, selected),
		blue.Told(player, selected),
		yellow.Told(player, selected),
		hawk.Told(player, selection.Allow{}),
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
