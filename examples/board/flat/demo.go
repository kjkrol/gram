package main

import (
	"embed"
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/bridges/site"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 204
	GridHeight   = 106
	CellSize     = 50
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
	EntitySize   = 20
	UnitSpeed    = CellSize * 3
	MaxEntCount  = 128
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world      *world.Plugin
	board      *board.Plugin
	bridges    *bridges.Plugin
	nav        *navigation.Plugin
	driving    *driving.Plugin
	collision  *collision.Plugin
	selection  *selection.Plugin
	players    *players.Plugin
	telemetry  *telemetry.Plugin
	cameras    *cameras.Plugin
	player     *players.Player
	vision     *vision.Plugin
	atmosphere *atmosphere.Plugin
}

func main() {
	props := game.Props{
		Title:       "gram board",
		Description: "Madeira on the simple map.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BoardStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  EntitySize,
				MaxSize:  CellSize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Rules(a.defineRules).
		Kinds(a.defineCells, a.defineKinds).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Shows(MainScene, telemetry.SceneName).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.collision = collision.NewPlugin(a.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithCollision(a.collision).
		WithLog(log.Default())
	a.bridges = bridges.NewPlugin(a.world, a.board, bridges.Config{
		Rules: site.Rules{Longest: 3},
	})
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.selection, a.driving).
		WithCollision(a.collision)
	a.vision = vision.NewPlugin(a.world).WithBoard(a.board)
	a.atmosphere = atmosphere.NewPlugin(
		a.world,
		atmosphere.Config{
			Calendar: calendar.Config{Start: 8 * time.Hour, Season: calendar.Summer},
			Climate:  climate.Config{Zone: climate.Temperate, Seed: uint64(time.Now().UnixNano())},
		},
	).WithWeathering(a.board, a.defineClimate())
	a.cameras = cameras.NewPlugin(a.world)
	a.telemetry = telemetry.NewPlugin(a.world).With(a.atmosphere.Reporter())
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.selection, a.nav, a.driving,
		a.atmosphere, a.vision, a.bridges)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineRules() {
	a.world.Roles().Define(MortalRole, act.Branch[unit.Standing]{
		Name: "drown",
		When: cond.Self.If(unit.Fallen()),
		Then: act.Self.Do(world.Despawn{}),
	})
}

func (a *arena) defineCells() {
	kinds := a.board.CellKinds()
	ground := cell.Land | cell.Air
	wet := cell.Water | cell.Air
	kinds.Define(WaterCell, cell.Kind{Cost: 1, Allows: wet})
	kinds.Define(EarthCell, cell.Kind{Cost: 2.5, Allows: ground}.Costing(cell.Air, 1))
	kinds.Define(SandCell, cell.Kind{Cost: 4, Allows: ground}.Costing(cell.Air, 1))
	kinds.Define(RockCell, cell.Kind{Cost: 3.25, Allows: ground}.Costing(cell.Air, 1))
	kinds.Define(BrookCell, cell.Kind{Cost: 3.25, Allows: cell.Land | wet}.Costing(wet, 1))
	kinds.Define(StreamCell, cell.Kind{Cost: 5, Allows: cell.Land | wet}.Costing(wet, 1))
	kinds.Define(RiverCell, cell.Kind{Cost: 1, Allows: wet})
	kinds.Define(RoadCell, cell.Kind{Cost: 1, Allows: ground, Graded: true})
	kinds.Define(ForestCell, cell.Kind{Cost: 7.5, Allows: ground, Veil: 0.6}.Costing(cell.Air, 1))
	a.defineWinterCells()
}

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

type fordRow struct {
	At  cell.ID `yaml:"at"`
	Way [2]int  `yaml:"way"`
}

func (a *arena) defineKinds() {
	brd := a.board.Board()
	units := board.NewUnits[unitRow](
		a.board,
		board.Shape{Size: EntitySize},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.Start) },
	)
	order := comp.Load(
		func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.Target} },
	)
	units.Define(
		UnitKind,
		unit.Mover{Domain: cell.Land},
		steering.Steering{
			MaxSpeed: UnitSpeed,
			Accel:    UnitSpeed * 2,
			Brake:    UnitSpeed * 4,
			V0:       UnitSpeed / 2,
			TurnRate: 0.15,
		},
		order,
		comp.Const(
			vision.Sight{Facing: geom.NewVec(1, 0), Radius: 7 * CellSize, Ahead: true},
		),
		comp.Const(world.Eye{Angle: 2 * math.Pi / 5}),
		comp.Const(vision.SightOutline{}),
		ceremony.Plays(a.world.Roles().Named(MortalRole)),
	)
	a.bridges.Types().Define(BridgeKind, bridges.Spec{
		Allows: cell.Land,
		Width:  48,
		Cost:   1,
		Rail:   cell.Rail{Keeps: cell.Land},
	})
	fords := board.NewEntities(a.board, board.Shape{Size: CellSize},
		func(r fordRow) cell.ID { return r.At })
	fords.Define(FordKind,
		comp.Load(func(r fordRow) world.Velocity {
			return world.Velocity{Dir: geom.NewVec(float64(r.Way[0]), float64(r.Way[1]))}
		}),
		comp.Const(cell.Floor{Allows: cell.Land, Cost: 6.25}))
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	if err := board.Spawn[fordRow](level, nil); err != nil {
		return err
	}
	if err := a.bridges.Spawn(level); err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{Selected: true}},
	})
}

func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).
		Input(a.players.Handle).
		Issue(a.players.IssueAs(a.player))}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.bridges.RunPlan(ctx, d)
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.nav.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.vision.RunPlan(ctx, d)
	a.atmosphere.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
