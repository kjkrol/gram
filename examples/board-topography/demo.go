package main

import (
	"embed"
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/atmosphere"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/atmosphere/climate"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
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
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 192
	GridHeight   = 96
	CellSize     = 32
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1024
	ScreenHeight = 768
	spritePx     = 22
	Sprint       = 4
	MaxEntCount  = 160
)

//go:embed map.yaml
var mapFile embed.FS

// A cell is 333 m of Madeira; the figures on it are drawn 50 times bigger than life.
var scale = world.Scale{Metres: 333.0 / CellSize, Figures: 50}

// unitSpeed is a demo's pace, three quarters of a cell a second: a man walks 1.4 m/s.
var unitSpeed = scale.Units(250)

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
	rival      *players.Player
	vision     *vision.Plugin
	atmosphere *atmosphere.Plugin
}

func main() {
	props := game.Props{
		Title:       "gram board topography",
		Description: "Madeira in relief.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(BoardTopographyStage).
		World(world.Config{
			Scale: scale,
			Space: world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  2,
				MaxSize:  CellSize,
			},
			Heights: true,
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayers).
		Effects(a.defineEffects).
		Rules(a.defineRules).
		Commands(a.defineCommands).
		Kinds(a.defineCells, a.defineKinds).
		Controls(a.bindKeys).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.collision = collision.NewPlugin(s.world)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).
		WithCollision(s.collision).
		WithLog(log.Default())
	s.topography = topography.NewPlugin(s.world, s.board, topography.Config{
		Cell:        CellSize,
		HeightUnit:  1,
		Perspective: true,
		Shaping: topography.Shaping{
			Step:    scale.Units(40),
			MaxStep: scale.Units(160),
		}})
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision).
		WithSpacing(navigation.BodySpacing)
	s.vision = vision.NewPlugin(s.world).WithBoard(s.board).WithGroundStep(scale.Units(50))
	s.atmosphere = atmosphere.NewPlugin(s.world, atmosphere.Config{
		Calendar: calendar.Config{
			Start:  18 * time.Hour,
			Season: calendar.Summer,
			Year:   calendar.EarthYear,
		},
		Climate: climate.Config{
			Zone:     climate.Mediterranean,
			Weathers: weathers,
			Start:    weather.Clear,
			Seed:     uint64(time.Now().UnixNano()),
		},
		Running: &atmosphere.Running{
			Day:        true,
			Weather:    true,
			Wind:       true,
			Clouds:     true,
			Falls:      true,
			Weathering: true,
			Stars:      true,
			Moon:       true,
		},
	})
	s.atmosphere.WithWeathering(s.board, s.defineClimate())
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving,
		s.atmosphere, s.topography, s.vision)
}

func (s *arena) definePlayers() error {
	s.player = s.players.Local("player")
	s.rival = s.players.Add("rival")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineEffects() {
	night := s.atmosphere.Calendar().Config().DayRealTime / 2
	s.world.Effects().Define(BloodMoonEf, effect.Spec{effect.Lasts(night),
		effect.Alter(func(m *sky.Moon) {
			m.Color, m.Face = render.Light{1, 0.25, 0.2}, render.Light{1, 0.3, 0.25}
			m.Strength *= 2
		})})
}

func (s *arena) defineRules() {
	s.world.Roles().Define(MortalRole,
		rule.Then[unit.Standing](
			"drown",
			rule.All,
			rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{})),
		))
	bloodMoon := s.world.Effects().Named(BloodMoonEf)
	s.world.Roles().Define(LunarRole,
		rule.Then[sky.Moonrise](
			"a blood moon rises",
			rule.All,
			rule.If(sky.Moonrise.Full, rule.Apply(bloodMoon)),
		))
	s.atmosphere.Plays(s.world.Roles().Named(LunarRole))
}

func (s *arena) defineCommands() {
	bleed := rule.Toggle(s.world.Effects().Named(BloodMoonEf)).On(s.atmosphere)
	s.world.Commands().Define(BleedCmd, bleed)
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
	kinds.Define(ForestCell, cell.Kind{
		Cost:   7.5,
		Allows: ground,
		Veil:   0.6,
		Height: scale.Units(20),
	}.Costing(cell.Air, 1))
	s.defineWinterCells()
}

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

type crossingRow struct {
	At     cell.ID `yaml:"at"`
	Height float64 `yaml:"height"` // metres over the cell's level
	Way    [2]int  `yaml:"way"`
}

// railed are the sides of a crossing's cell along its way, where its railings stand.
func (r crossingRow) railed() cell.Links {
	if r.Way[1] == 0 {
		return 1 | 2 // running east and west: north and south
	}
	return 4 | 8
}

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[unitRow](
		s.board,
		board.Shape{Size: scale.Figure(0.6), Height: scale.Figure(1.8)},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.Start) },
	)
	order := comp.Load(
		func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.Target} },
	)
	sight := comp.Const(vision.Sight{
		Facing: geom.NewVec(1, 0),
		Radius: scale.Units(10000),
		Ahead:  true,
	})
	eye := comp.Const(world.Eye{Angle: 72 * math.Pi / 180})
	walker := steering.Steering{
		MaxSpeed: unitSpeed,
		Sprint:   Sprint,
		Accel:    unitSpeed * 2,
		Brake:    unitSpeed * 4,
		V0:       unitSpeed / 2,
		TurnRate: 0.15,
	}
	mortal := rule.Plays(s.world.Roles().Named(MortalRole))
	units.Define(UnitKind, unit.Mover{Domain: cell.Land}, walker,
		order,
		sight, eye, mortal,
	)
	units.Define(PlateauKind, unit.Mover{Domain: cell.Land}, walker,
		sight, eye, mortal,
	)
	units.Define(RivalKind, unit.Mover{Domain: cell.Land}, walker,
		order,
		sight, eye, mortal,
	)
	units.Define(HawkKind,
		unit.Mover{
			Domain:    cell.Air,
			Lift:      scale.Units(300),
			Clearance: scale.Units(20),
			Ceiling:   scale.Units(air.CloudBase - 100),
		},
		steering.Steering{
			MaxSpeed: unitSpeed * 1.5,
			Sprint:   Sprint,
			Accel:    unitSpeed * 2,
			Brake:    unitSpeed * 4,
			V0:       unitSpeed / 2,
			TurnRate: 0.1},
		order,
		sight, eye,
	)
	crossings := board.NewEntities(s.board, board.Shape{Size: CellSize, Height: scale.Units(2)},
		func(r crossingRow) cell.ID { return r.At })
	way := comp.Load(func(r crossingRow) world.Velocity {
		return world.Velocity{Dir: geom.NewVec(float64(r.Way[0]), float64(r.Way[1]))}
	})
	crossings.Define(BridgeKind, way,
		comp.Load(func(r crossingRow) cell.Floor {
			return cell.Floor{
				Allows:    cell.Land,
				Height:    scale.Units(r.Height),
				Thickness: scale.Units(2),
				Cost:      1,
				Graded:    true,
			}
		}),
		comp.Load(func(r crossingRow) cell.Rail {
			return cell.Rail{Keeps: cell.Land, Sides: r.railed(), Height: scale.Figure(1)}
		}))
	crossings.Define(FordKind, way,
		comp.Const(cell.Floor{Allows: cell.Land, Cost: 6.25}))
}

func (s *arena) bindKeys() error {
	return s.player.Bind(
		control.Give(
			control.KeyPress{Key: control.KeyM},
			"Blood moon, on or off",
			s.world.Commands().Named(BleedCmd),
		),
		cameras.MouseLookKey(
			control.KeyO,
		), // first person looks round with the mouse only once asked
	)
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	s.topography.Seed(level.Heights())
	if err := board.Spawn[crossingRow](level, nil); err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide:  {players.Give{To: s.player.ID}, selection.Allow{Selected: true}},
		RivalSide:   {players.Give{To: s.rival.ID}, selection.Allow{}},
		ReserveSide: {players.Give{To: s.player.ID}, selection.Allow{}},
	})
}

func (s *arena) defineScenes(ctx game.Initializer) []game.Scene {
	main := &mainScene{arena: s, tps: ctx.TPS()}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
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
	s.topography.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
