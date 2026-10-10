package main

import (
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/examples/island"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
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
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	CellSize     = island.CellSize
	WorldWidth   = island.GridWidth * CellSize
	WorldHeight  = island.GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
	EntitySize   = 22
	UnitSpeed    = CellSize * 3
	sightRadius  = 220
	sightHalf    = math.Pi / 5
	MaxEntCount  = 4 * island.Stops
)

type arena struct {
	world      *world.Plugin
	board      *board.Plugin
	nav        *navigation.Plugin
	driving    *driving.Plugin
	collision  *collision.Plugin
	selection  *selection.Plugin
	players    *players.Plugin
	cameras    *cameras.Plugin
	player     *players.Player
	vision     *vision.Plugin
	atmosphere *atmosphere.Plugin
	// TODO: ten sposob definiowania startowych celow jest bardzo hacky
	stops []cell.ID // where the units start, as the layout says
}

func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram board",
		Description: "The island on the simple map.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(BoardStage).
			World(world.Config{
				Space: world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
				Entities: world.EntitiesCfg{
					MaxCount: MaxEntCount,
					MinSize:  EntitySize,
					MaxSize:  EntitySize,
				},
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayer).
			Effects(s.defineEffects).
			Rules(s.defineRules).
			Kinds(s.defineCells, s.defineKinds).
			Spawn(s.spawnCells, s.spawnUnits).
			Scenes(s.defineScenes).
			Update(s.update))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(island.GridWidth, island.GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).
		WithCollision(s.collision).
		WithLog(log.Default())
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision)
	s.vision = vision.NewPlugin(s.world).WithBoard(s.board).
		WithViews(render.Show(s.selection.IsSelected))
		// TODO: render tutaj? to dziwne. nie wiem co to robi, ale na oko, to powinno byc w scene
	s.atmosphere = atmosphere.NewPlugin(
		s.world,
		atmosphere.Config{
			Calendar: calendar.Config{Season: calendar.Autumn},
			Climate:  climate.Config{Zone: climate.Temperate, Seed: uint64(time.Now().UnixNano())},
		},
	)
	// TODO: raczej plansza z atmosfera, a nie atmosfera z planszą...
	s.atmosphere.WithBoard(s.board)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving,
		s.atmosphere, s.vision)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineCells() {
	island.Define(s.board.CellKinds(), 0)
	s.defineWinterCells()
}

func (s *arena) defineEffects() { s.atmosphere.WithWeathering(s.board, s.defineClimate()) }

func (s *arena) defineRules() {
	s.world.Roles().Define(MortalRole,
		rule.Then[unit.Standing](
			"drown",
			rule.All,
			rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{})),
		))
}

func (s *arena) defineScenes(ctx game.Initializer) []game.Scene {
	main := &mainScene{arena: s, tps: ctx.TPS()}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
}

type unitRow struct{ start, target cell.ID }

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[unitRow](
		s.board,
		board.Shape{Size: EntitySize},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.start) },
	)
	order := comp.Load(
		func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.target} },
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
			vision.Sight{Facing: geom.NewVec(1, 0), Radius: sightRadius, Ahead: true},
		),
		comp.Const(world.Eye{Angle: 2 * sightHalf}),
		comp.Const(vision.SightOutline{}),
		rule.Plays(s.world.Roles().Named(MortalRole)),
	)
}

func (s *arena) spawnCells() {
	layout, _, stops := island.Layout(s.board.Board())
	s.stops = stops
	s.board.Seed(layout)
}

func (s *arena) spawnUnits() {
	unitKind := kind.Named[unitRow](s.world.Kinds(), UnitKind)
	entries := make([]kind.Entry, 0, len(s.stops))
	for i, from := range s.stops {
		to := s.stops[(i+len(s.stops)/2)%len(s.stops)]
		walker := unitKind.Entry(unitRow{start: from, target: to})
		walker = walker.Told(players.Give{To: s.player.ID}, selection.Allow{Selected: true})
		entries = append(entries, walker)
	}
	s.world.Seed(entries...)
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
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
