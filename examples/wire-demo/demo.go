package main

import (
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
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
	MaxEntCount  = 16
)

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

func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram wire demo",
		Description: "Wires: a lever, a plate and a switch.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(WireStage).
			World(world.Config{
				Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
				Entities: world.EntitiesCfg{
					MaxCount: MaxEntCount,
					MinSize:  EntitySize,
					MaxSize:  EntitySize,
				},
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayer).
			Effects(s.defineEffects).
			Rules(s.defineRoles).
			Commands(s.defineCommands).
			Kinds(s.defineCells, s.defineKinds).
			Controls(s.bindKeys).
			Spawn(s.spawnCells, s.spawnUnits).
			Scenes(s.defineScenes).
			Update(s.update))
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

func (s *arena) defineCells() {
	kinds, roles := s.board.CellKinds(), s.world.Roles()
	kinds.Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(BoardsCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(PitCell, cell.Kind{Cost: 1})
	kinds.Define(PlateCell, cell.Kind{Cost: 1, Allows: cell.Land}, roles.Named(PlateRole))
	kinds.Define(LeverCell, cell.Kind{Cost: 1, Allows: cell.Land}, roles.Named(LeverRole))
	kinds.Define(FenceCell, cell.Kind{Cost: 1, Solid: true})
	kinds.Define(GateCell, cell.Kind{Cost: 1, Solid: true})
	kinds.Define(GatewayCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

func (s *arena) defineEffects() {
	kinds := s.board.CellKinds()
	fx := s.world.Effects()
	fx.Define(
		OpenEf,
		effect.Spec{
			effect.Lasts(2 * time.Second),
			effect.Alter(func(g *cell.Ground) { g.Kind = kinds.Named(PitCell).Kind() }),
		},
	)
	fx.Define(
		AjarEf,
		effect.Spec{
			effect.Alter(func(g *cell.Ground) { g.Kind = kinds.Named(GatewayCell).Kind() }),
		},
	)
	fx.Define(HasteEf, effect.Spec{
		effect.Lasts(3 * time.Second),
		effect.Alter(
			func(st *steering.Steering) { st.MaxSpeed, st.Accel = st.MaxSpeed*2, st.Accel*2 },
		),
	})
	fx.Define(PullEf, effect.Spec{effect.Lasts(time.Second / 4)})
}

func (s *arena) defineRoles() {
	s.world.Roles().Define(PlateRole,
		rule.Then[cell.Now]("press", rule.All, rule.If(cell.Now.Stood, rule.Trigger())))
	s.world.Roles().Define(LeverRole)
	s.world.Roles().Define(HastyRole)
	s.world.Roles().Define(HandyRole,
		rule.Then[unit.Standing](
			"pull the lever beside",
			rule.All,
			rule.Under(
				s.world.Effects().Named(PullEf),
				rule.Around(1, rule.Playing(s.world.Roles().Named(LeverRole), rule.Trigger())),
			),
		))
	s.world.Roles().Define(MortalRole,
		rule.Then[unit.Standing](
			"fall in",
			rule.All,
			rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{})),
		))
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

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
}

type unitRow struct{ start, to cell.ID }

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[unitRow](
		s.board,
		board.Shape{Size: EntitySize},
		func(u unitRow) geom.Vec { return brd.CellCenter(u.start) },
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
	// a wanderer walks to the other end of its row and back, a second's rest at each end
	units.Define(
		WandererKind,
		land,
		profile,
		rule.Plays(s.world.Roles().Named(MortalRole)),
		comp.Load(func(u unitRow) navigation.MoveOrder {
			return navigation.Patrol(time.Second, u.to, u.start)
		}),
	)
}

// TODO: z pliku...
func (s *arena) spawnCells() {
	brd := s.board.Board()
	var cells []cell.Entry
	strips := []struct {
		group string
		left  uint32
	}{{"west trapdoors", 7}, {"east trapdoors", 15}}
	for _, strip := range strips {
		for y := uint32(1); y <= 11; y++ {
			for x := strip.left; x <= strip.left+1; x++ {
				door := cell.Entry{Kind: BoardsCell, Cell: brd.At(x, y), Group: strip.group}
				cells = append(cells, door)
			}
		}
	}
	for x := range uint32(GridWidth) {
		if x == 11 || x == 12 {
			cells = append(cells, cell.Entry{Kind: GateCell, Cell: brd.At(x, 13), Group: "gate"})
		} else {
			cells = append(cells, cell.Entry{Kind: FenceCell, Cell: brd.At(x, 13)})
		}
	}
	cells = append(cells, cell.Entry{Kind: PlateCell, Cell: brd.At(GridWidth-4, 14), Name: "plate"})
	cells = append(cells, cell.Entry{Kind: LeverCell, Cell: brd.At(1, 14), Name: "west lever"})
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}

// TODO: z pliku
func (s *arena) spawnUnits() {
	brd := s.board.Board()
	scoutKind := kind.Named[unitRow](s.world.Kinds(), ScoutKind)
	porterKind := kind.Named[unitRow](s.world.Kinds(), PorterKind)
	wandererKind := kind.Named[unitRow](s.world.Kinds(), WandererKind)
	player := []control.Command{players.Give{To: s.player.ID}, selection.Allow{}}
	for i := range uint32(3) {
		s.world.Seed(scoutKind.Entry(unitRow{start: brd.At(3+2*i, 14)}).Told(player...))
	}
	for i := range uint32(2) {
		s.world.Seed(porterKind.Entry(unitRow{start: brd.At(4+2*i, 15)}).Told(player...))
	}
	for _, row := range []uint32{2, 5, 8, 11} {
		s.world.Seed(
			wandererKind.Entry(unitRow{start: brd.At(2, row), to: brd.At(GridWidth-3, row)}),
		)
	}
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
