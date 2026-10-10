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
	TPS         = 60
	GridWidth   = 20
	GridHeight  = 12
	HexSize     = 24
	EntitySize  = 22
	UnitSpeed   = HexSize * 3
	MaxEntCount = 32
)

var (
	ScreenWidth = int(
		math.Ceil(HexSize*(math.Sqrt(3)*(GridWidth-1)+math.Sqrt(3)/2*(GridHeight-1)) + 2*HexSize),
	)
	ScreenHeight = int(math.Ceil(HexSize * (1.5*(GridHeight-1) + 2)))
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
		Title:       "gram navigation vision hex demo",
		Description: "Sight across a hex board: walls cut, forests dim, a hawk flies over.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(BoardNavigationVisionHexStage).
			World(world.Config{
				Space: world.SpaceCfg{Width: uint32(ScreenWidth), Height: uint32(ScreenHeight)},
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
	grid := grid.DefaultGrids{}.Hex(GridWidth, GridHeight, HexSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision)
	s.topography = topography.NewPlugin(s.world, s.board, topography.Config{Cell: HexSize})
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
	// Every unit is 2 tall; the eye is a fact of the kind, the altitude the board's to write.
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
	// The hawk flies 40 above the ground on the Air plane: walls and walkers pass under it, and its
	// eye looks over the wall, the forest and the hill that stop a walker's.
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

// TODO: to powinno byc z pliku, naturalnie tu format jest troche inny, bo uwzgleniamy "z"
func (s *arena) spawnCells() {
	brd := s.board.Board()
	// A wall down the q = 10 column with a gap at r = 6, a forest either side of the gap, and a
	// road along r = 0 with both flanks.
	var cells []cell.Entry
	for r := uint32(1); r < GridHeight; r++ {
		if r == 6 {
			continue
		}
		cells = append(cells, cell.Entry{Kind: WallCell, Cell: brd.At(10, r)})
	}
	for _, f := range [][2]uint32{{5, 4}, {13, 8}} {
		for dr := uint32(0); dr < 3; dr++ {
			for dq := uint32(0); dq < 3; dq++ {
				cells = append(cells, cell.Entry{Kind: ForestCell, Cell: brd.At(f[0]+dq, f[1]+dr)})
			}
		}
	}
	// A hill in the first unit's way: its cone climbs the slope and stops, the hawk's passes over.
	for dr := uint32(2); dr <= 4; dr++ {
		for dq := uint32(8); dq <= 10; dq++ {
			if dq != 10 {
				cells = append(cells, cell.Entry{Kind: HillCell, Cell: brd.At(dq, dr)})
			}
		}
	}
	for q := uint32(2); q <= GridWidth-3; q++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.At(q, 0)})
	}
	for r := uint32(1); r < GridHeight; r++ {
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.At(2, r)})
		cells = append(cells, cell.Entry{Kind: RoadCell, Cell: brd.At(GridWidth-3, r)})
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
	red := scoutKind(0).Entry(unitRow{start: brd.At(3, 3), target: brd.At(GridWidth-4, 3)})
	blue := scoutKind(1).Entry(unitRow{start: brd.At(3, 9), target: brd.At(GridWidth-4, 9)})
	yellow := scoutKind(2).Entry(unitRow{start: brd.At(GridWidth-4, 6), target: brd.At(3, 6)})
	// The hawk crosses the wall and the second forest head-on.
	hawk := hawkKind.Entry(unitRow{start: brd.At(1, 9), target: brd.At(GridWidth-2, 9)})
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
