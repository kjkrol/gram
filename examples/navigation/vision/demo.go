package main

import (
	"embed"
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
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

//go:embed map.yaml
var mapFile embed.FS

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

func main() {
	props := game.Props{
		Title:       "gram navigation vision demo",
		Description: "Sight across a board: walls cut, forests dim, a hawk flies over.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BoardNavigationVisionStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  EntitySize,
				MaxSize:  EntitySize,
			},
			Heights: true,
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Kinds(a.defineCells, a.defineKinds).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.collision = collision.NewPlugin(a.world)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).WithCollision(a.collision)
	a.topography = topography.NewPlugin(a.world, a.board, topography.Config{Cell: CellSize})
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.selection, a.driving).
		WithCollision(a.collision)
	a.vision = vision.NewPlugin(a.world).WithBoard(a.board).WithLog(log.Default())
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(
		a.world,
		a.collision, a.cameras,
		a.board,
		a.selection,
		a.nav,
		a.driving,
		a.topography,
		a.vision,
	)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineCells() {
	kinds := a.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1))
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true, Allows: cell.Air, Veil: 1, Height: 10})
	kinds.Define(
		ForestCell,
		cell.Kind{Cost: 3, Allows: cell.Land | cell.Air, Veil: 0.6, Height: 8}.Costing(cell.Air, 1),
	)
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: cell.Land | cell.Air})
	kinds.Define(HillCell, cell.Kind{Cost: 2, Allows: cell.Land | cell.Air}.Costing(cell.Air, 1))
}

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

func (a *arena) defineKinds() {
	brd := a.board.Board()
	units := board.NewUnits[unitRow](
		a.board,
		board.Shape{Size: EntitySize, Height: 2},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.Start) },
	)
	order := comp.Load(
		func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.Target} },
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

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	a.topography.Seed(level.Heights())
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide:  {players.Give{To: a.player.ID}, selection.Allow{Selected: true}},
		ReserveSide: {players.Give{To: a.player.ID}, selection.Allow{}},
	})
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.nav.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.vision.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.topography.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
