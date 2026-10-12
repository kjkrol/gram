package main

import (
	"embed"
	"errors"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
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
		Title:       "gram wire demo",
		Description: "Wires: a lever, a plate and a switch.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(WireStage).
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
	fx := s.world.Effects()
	fx.Define(OpenEf, effect.Spec{
		effect.Lasts(2 * time.Second),
		effect.Alter(func(f *cell.Floor) { f.Allows = 0 }),
	})
	fx.Define(AjarEf, effect.Spec{effect.Alter(func(w *cell.Wall) { w.Keeps = 0 })})
	fx.Define(HasteEf, effect.Spec{
		effect.Lasts(3 * time.Second),
		effect.Alter(
			func(st *steering.Steering) { st.MaxSpeed, st.Accel = st.MaxSpeed*2, st.Accel*2 },
		),
	})
	fx.Define(PullEf, effect.Spec{effect.Lasts(time.Second / 4)})
}

func (s *arena) defineRoles() {
	roles := s.world.Roles()
	roles.Define(PlateRole, rule.Branch[collision.Meeting]{Name: "press", Do: rule.Trigger()})
	roles.Define(LeverRole)
	roles.Define(HastyRole)
	roles.Define(HandyRole, rule.Branch[unit.Standing]{
		Name: "pull the lever beside",
		When: rule.Under(s.world.Effects().Named(PullEf)),
		Do: rule.Around(1, rule.Branch[any]{
			When: rule.Playing(roles.Named(LeverRole)),
			Do:   rule.Trigger(),
		}),
	})
	roles.Define(MortalRole, rule.Branch[unit.Standing]{
		Name: "fall in",
		When: unit.Standing.Fallen,
		Do:   rule.Order(world.Despawn{}),
	})
}

func (s *arena) defineCommands() {
	fx, roles, cmds := s.world.Effects(), s.world.Roles(), s.world.Commands()
	cmds.Define(
		OpenWestCmd,
		rule.Cast(fx.Named(OpenEf)).
			On(entity.Group("west trapdoors")).
			By(entity.Named("west lever")),
	)
	cmds.Define(
		OpenEastCmd,
		rule.Cast(fx.Named(OpenEf)).On(entity.Group("east trapdoors")).By(entity.Named("plate")),
	)
	cmds.Define(FlipTheGateCmd, rule.Toggle(fx.Named(AjarEf)).On(entity.Group("gate")))
	cmds.Define(
		HastenCmd,
		rule.Cast(fx.Named(HasteEf)).On(s.selection.Selected(roles.Named(HastyRole))),
	)
	cmds.Define(
		ReachCmd,
		rule.Cast(fx.Named(PullEf)).On(s.selection.Selected(roles.Named(HandyRole))),
	)
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(PitCell, cell.Kind{Cost: 1})
	kinds.Define(FenceCell, cell.Kind{Cost: 1, Solid: true})
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
	laden := steering.Steering{
		MaxSpeed: UnitSpeed * 3 / 4,
		Accel:    UnitSpeed,
		V0:       UnitSpeed / 4,
		TurnRate: 0.1,
	}
	land := unit.Mover{Domain: cell.Land}
	units.Define(
		ScoutKind,
		land,
		profile,
		rule.Plays(
			s.world.Roles().Named(HastyRole),
			s.world.Roles().Named(HandyRole),
			s.world.Roles().Named(MortalRole),
		),
	)
	units.Define(PorterKind, land, laden, rule.Plays(s.world.Roles().Named(MortalRole)))
	units.Define(
		WandererKind,
		land,
		profile,
		rule.Plays(s.world.Roles().Named(MortalRole)),
		comp.Load(func(u unitRow) navigation.MoveOrder {
			return navigation.Patrol(time.Second, u.To, u.Start)
		}),
	)
	at := func(r standingRow) cell.ID { return r.At }
	cellWide := board.NewEntities(s.board, board.Shape{Size: CellSize}, at)
	cellWide.Define(TrapdoorKind, comp.Const(cell.Floor{Allows: cell.Land}))
	cellWide.Define(GateKind, comp.Const(cell.Wall{Keeps: cell.Land}))
	small := board.NewEntities(s.board, board.Shape{Size: EntitySize}, at)
	small.Define(PlateKind,
		comp.Const(collision.Collider{}),
		rule.Plays(s.world.Roles().Named(PlateRole)))
	small.Define(LeverKind, rule.Plays(s.world.Roles().Named(LeverRole)))
}

func (s *arena) bindKeys() error {
	return s.player.Bind(
		control.Give(
			control.KeyPress{Key: control.Key1},
			"Pull the west lever: its trapdoors open for a while",
			s.world.Commands().Named(OpenWestCmd),
		),
		control.Give(
			control.KeyPress{Key: control.KeyG},
			"Flip the gate's switch: open, or shut",
			s.world.Commands().Named(FlipTheGateCmd),
		),
		control.Give(
			control.KeyPress{Key: control.KeyJ},
			"Hasten the selected scouts",
			s.world.Commands().Named(HastenCmd),
		),
		control.Give(
			control.KeyPress{Key: control.KeyU},
			"Pull the lever beside the selected scouts",
			s.world.Commands().Named(ReachCmd),
		),
	)
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
