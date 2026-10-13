package main

import (
	"embed"
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/entity/kind/comp"
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
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/act"
	"github.com/kjkrol/gram/rule/cond"
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
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).
		WithCollision(s.collision).
		WithLog(log.Default())
	s.bridges = bridges.NewPlugin(s.world, s.board, bridges.Config{
		Rules: site.Rules{Longest: 3},
	})
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision)
	s.vision = vision.NewPlugin(s.world).WithBoard(s.board)
	s.atmosphere = atmosphere.NewPlugin(
		s.world,
		atmosphere.Config{
			Calendar: calendar.Config{Start: 8 * time.Hour, Season: calendar.Summer},
			Climate:  climate.Config{Zone: climate.Temperate, Seed: uint64(time.Now().UnixNano())},
		},
	).WithWeathering(s.board, s.defineClimate())
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving,
		s.atmosphere, s.vision, s.bridges)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineRules() {
	s.world.Roles().Define(MortalRole, act.Branch[unit.Standing]{
		Name: "drown",
		When: cond.Self.If(unit.Fallen()),
		Then: act.Self.Do(world.Despawn{}),
	})
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
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
	s.defineWinterCells()
}

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

type fordRow struct {
	At  cell.ID `yaml:"at"`
	Way [2]int  `yaml:"way"`
}

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[unitRow](
		s.board,
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
		rule.Plays(s.world.Roles().Named(MortalRole)),
	)
	s.bridges.Types().Define(BridgeKind, bridges.Spec{
		Allows: cell.Land,
		Width:  48,
		Cost:   1,
		Rail:   cell.Rail{Keeps: cell.Land},
	})
	fords := board.NewEntities(s.board, board.Shape{Size: CellSize},
		func(r fordRow) cell.ID { return r.At })
	fords.Define(FordKind,
		comp.Load(func(r fordRow) world.Velocity {
			return world.Velocity{Dir: geom.NewVec(float64(r.Way[0]), float64(r.Way[1]))}
		}),
		comp.Const(cell.Floor{Allows: cell.Land, Cost: 6.25}))
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	if err := board.Spawn[fordRow](level, nil); err != nil {
		return err
	}
	if err := s.bridges.Spawn(level); err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: s.player.ID}, selection.Allow{Selected: true}},
	})
}

func (s *arena) defineScenes(ctx game.Initializer) []game.Scene {
	main := &mainScene{arena: s, tps: ctx.TPS()}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).
		Input(s.players.Handle).
		Issue(s.players.IssueAs(s.player))}
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.bridges.RunPlan(ctx, d)
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.driving.RunPlan(ctx, d)
	s.vision.RunPlan(ctx, d)
	s.atmosphere.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
