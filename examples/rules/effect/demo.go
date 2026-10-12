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
	"github.com/kjkrol/gram/rule/effect"
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
	MaxEntCount  = 16

	Frost = cell.Domain(1 << 3)
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
		Title:       "gram effect demo",
		Description: "An ice witch writes the terrain.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(EffectStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  EntitySize,
				MaxSize:  EntitySize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRoles).
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
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
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
	effects := s.world.Effects()
	effects.Define(FrostEf, effect.Spec{
		effect.Described("Land under snow: slower, and the witch's own ground."),
		effect.Lasts(5 * time.Second),
		effect.Alter(func(g *cell.Ground) {
			g.Kind.Allows |= Frost
			g.Kind.Cost++
			g.Kind = g.Kind.Costing(Frost, 0.5)
		}),
	})
	effects.Define(IcedEf, effect.Spec{
		effect.Described("Water under ice: walked over, not sailed."),
		effect.Lasts(5 * time.Second),
		effect.Alter(func(g *cell.Ground) {
			g.Kind.Allows = cell.Land | Frost
			g.Kind.Cost = 2
			g.Kind = g.Kind.Costing(Frost, 0.5)
		}),
	})
	effects.Define(FrozenEf, effect.Spec{
		effect.Described("Frozen stiff: stood dead still, an immovable block."),
		effect.Alter(func(p *collision.Physics) { p.Mass = math.Inf(1) }),
		effect.Alter(func(st *steering.Steering) { st.Halted = true }),
	})
	effects.Define(SlipEf, effect.Spec{
		effect.Described("On the ice: hardly any braking, whoever moves slides on."),
		effect.Alter(func(st *steering.Steering) { st.Brake = st.Accel / 8 }),
	})
}

func (s *arena) defineRoles() {
	effects := s.world.Effects()
	frost, iced := effects.Named(FrostEf), effects.Named(IcedEf)
	frozen, slip := effects.Named(FrozenEf), effects.Named(SlipEf)

	roles := s.world.Roles()
	roles.Define(LakeRole)
	roles.Define(WitchRole,
		rule.Then[unit.Standing]("freeze", rule.All, rule.Around(1, rule.OneOf(
			rule.Playing(roles.Named(LakeRole), rule.Apply(iced)),
			rule.Apply(frost),
		))),
	)
	roles.Define(MortalRole,
		rule.Then[unit.Standing]("fallen in", rule.All,
			rule.If(unit.Standing.Fallen, rule.OneOf(
				rule.If(unit.Over(iced), rule.Keep(frozen)),
				rule.Order(world.Despawn{}),
			))),
		rule.Then[unit.Standing]("on the ice", rule.All,
			rule.If(rule.Not(unit.Standing.Fallen), rule.If(unit.Over(iced), rule.Keep(slip)))),
	)
}

func (s *arena) defineCommands() {
	s.world.Commands().Define(FreezeCmd,
		rule.Cast(s.world.Effects().Named(FrozenEf)).On(s.selection.Pointed()).For(3*time.Second))
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 2, Allows: cell.Land})
	kinds.Define(WaterCell, cell.Kind{Cost: 1, Allows: cell.Water})
}

type unitRow struct {
	Start  cell.ID `yaml:"start"`
	Target cell.ID `yaml:"target"`
}

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[unitRow](
		s.board,
		board.Shape{Size: EntitySize},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.Start) },
	)
	profile := func(brake float64) steering.Steering {
		return steering.Steering{
			MaxSpeed: UnitSpeed,
			Accel:    UnitSpeed * 2,
			Brake:    brake,
			V0:       UnitSpeed / 2,
			TurnRate: 0.15,
		}
	}
	order := comp.Load(
		func(u unitRow) navigation.MoveOrder { return navigation.MoveOrder{Target: u.Target} },
	)
	roles := s.world.Roles()
	units.Define(
		WitchKind,
		unit.Mover{Domain: cell.Land | cell.Water | Frost},
		profile(UnitSpeed*4),
		order,
		rule.Plays(roles.Named(WitchRole), roles.Named(MortalRole)),
	)
	units.Define(
		WalkerKind,
		unit.Mover{Domain: cell.Land},
		profile(UnitSpeed*4),
		rule.Plays(roles.Named(MortalRole)),
	)
	units.Define(
		BoatKind,
		unit.Mover{Domain: cell.Water},
		profile(UnitSpeed/4),
		order,
		rule.Plays(roles.Named(MortalRole)),
	)
}

func (s *arena) bindKeys() error {
	return s.player.Bind(
		control.Give(control.KeyPress{Key: control.KeyF}, "Freeze the one pointed at",
			s.world.Commands().Named(FreezeCmd)),
	)
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: s.player.ID}, selection.Allow{}},
	})
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
