package main

import (
	"embed"
	"errors"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/entity"
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
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/act"
	"github.com/kjkrol/gram/rule/cond"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/rule/plan"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
	EntitySize   = 20
	UnitSpeed    = CellSize * 2
	MaxEntCount  = 64
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	driving   *driving.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	player    *players.Player
}

func main() {
	props := game.Props{
		Title:       "gram pressure plate demo",
		Description: "Pressure plates and trapdoors.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(PressurePlateStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  EntitySize,
				MaxSize:  CellSize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRules).
		Commands(a.defineCommands).
		Kinds(a.defineCells, a.defineKinds).
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
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(OpenEf, effect.Spec{
		effect.Lasts(time.Second),
		effect.Alter(func(f *cell.Floor) { f.Allows = 0 }),
	})
}

func (s *arena) defineRules() {
	s.world.Roles().Define(PlateRole,
		act.Branch[collision.Meeting]{
			Name: "press",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(rule.Trigger{}),
		})
	s.world.Roles().Define(MortalRole, act.Branch[unit.Standing]{
		Name: "fall in",
		When: cond.Self.If(unit.Fallen()),
		Then: act.Self.Do(world.Despawn{}),
	})
}

func (s *arena) defineCommands() {
	for _, strip := range []string{"west", "east"} {
		s.world.Commands().Define(openCmd(strip),
			rule.Put(s.world.Effects().Named(OpenEf)).
				On(entity.Group("trapdoors "+strip)).
				By(entity.Named("plate "+strip)),
		)
	}
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(PitCell, cell.Kind{Cost: 1})
}

type unitRow struct {
	Start cell.ID `yaml:"start"`
	To    cell.ID `yaml:"to"`
}

type standingRow struct {
	At cell.ID `yaml:"at"`
}

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[unitRow](
		s.board,
		board.Shape{Size: EntitySize},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.Start) },
	)
	profile := steering.Steering{
		MaxSpeed: UnitSpeed,
		Accel:    UnitSpeed * 2,
		V0:       UnitSpeed / 2,
		TurnRate: 0.15,
	}
	land := unit.Mover{Domain: cell.Land}
	units.Define(ScoutKind, land, profile, rule.Plays(s.world.Roles().Named(MortalRole)))
	patrol := plan.New(PatrolPlan, plan.Steps(
		plan.Loop[any]{
			Name:  "leg",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(navigation.Onward{}),
			Until: cond.Self.If(navigation.Arrived{}),
		},
		plan.Loop[any]{Name: "rest", When: cond.Self.If(cond.Always), Then: plan.Self.Wait(time.Second)},
	))
	round := comp.Load(func(u unitRow) navigation.Round {
		return navigation.NewRound(u.To, u.Start)
	})
	units.Define(WandererKind, land, profile, rule.Plays(s.world.Roles().Named(MortalRole)),
		round, patrol)
	at := func(r standingRow) cell.ID { return r.At }
	doors := board.NewEntities(s.board, board.Shape{Size: CellSize}, at)
	doors.Define(TrapdoorKind, comp.Const(cell.Floor{Allows: cell.Land}))
	plates := board.NewEntities(s.board, board.Shape{Size: EntitySize}, at)
	plates.Define(PlateKind,
		comp.Const(collision.Collider{}),
		rule.Plays(s.world.Roles().Named(PlateRole)))
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	sides := board.Sides{
		PlayerSide: {players.Give{To: s.player.ID}, selection.Allow{}},
	}
	return errors.Join(
		board.Spawn[standingRow](level, nil),
		board.Spawn[unitRow](level, sides),
	)
}

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.board.RunPlan(ctx, d)
	s.nav.RunPlan(ctx, d)
	s.driving.RunPlan(ctx, d)
	s.selection.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
