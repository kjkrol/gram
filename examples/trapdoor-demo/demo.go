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

	// TODO: ops planszy tutaj?! serio? musi istniec przez caly przebieg gry tutaj? jprdle
	stripTop, stripBottom uint32 = 2, 13
)

// TODO: cala ta sekcja az do ---- powinna byc przeniesiona tam gdzie jest uzywna!!!
var rows = []uint32{3, 6, 9, 12}

var levers = []struct {
	name string
	key  control.Key
	left uint32
}{{"west", control.Key1, 7}, {"east", control.Key2, 15}}

type unitRow struct{ start, to cell.ID }

// ----

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
	brd       *board.Board
}

func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram trapdoor demo",
		Description: "Trapdoors under a lever.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(TrapdoorStage).
			World(world.Config{
				Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
				Entities: world.EntitiesCfg{MaxCount: MaxEntCount, MinSize: EntitySize, MaxSize: EntitySize},
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayer).
			Effects(s.defineEffects).
			Rules(s.defineRules).
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
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
	s.brd = s.board.Res.Logic.Board
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(BoardsCell, cell.Kind{Cost: 1, Allows: cell.Land}) // a trapdoor shut
	kinds.Define(PitCell, cell.Kind{Cost: 1})                       // holds nobody
}

func (s *arena) defineEffects() {
	kinds := s.board.CellKinds()
	fx := s.world.Effects()
	fx.Define(OpenEf, effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind = kinds.Named(PitCell).Kind() })})
	fx.Define(HasteEf, effect.Spec{
		effect.Lasts(2 * time.Second),
		effect.Alter(func(st *steering.Steering) { st.MaxSpeed, st.Accel = st.MaxSpeed*2, st.Accel*2 }),
	})
}

func (s *arena) defineRules() {
	s.world.Roles().Define(MortalRole,
		rule.Then[unit.Standing]("fall in", rule.All, rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))
}

func (s *arena) defineCommands() {
	cmds := s.world.Commands()
	for _, l := range levers {
		cmds.Define(pullCmd(l.name), rule.Cast(s.world.Effects().Named(OpenEf)).On(entity.Group("trapdoors "+l.name)).For(3*time.Second))
	}
	cmds.Define(HastenCmd, rule.Cast(s.world.Effects().Named(HasteEf)).On(s.selection.Selected()))
}

func (s *arena) bindKeys() error {
	for _, l := range levers {
		if err := s.player.Bind(control.Give(control.KeyPress{Key: l.key}, "Pull the "+l.name+" lever: its trapdoors open", s.world.Commands().Named(pullCmd(l.name)))); err != nil {
			return err
		}
	}
	return s.player.Bind(control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts", s.world.Commands().Named(HastenCmd)))
}

func (s *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(s.players.Handle)}
}

func (s *arena) defineKinds() {
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return s.brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, V0: UnitSpeed / 2, TurnRate: 0.15}
	land := unit.Mover{Domain: cell.Land}
	units.Define(ScoutKind, land, profile, rule.Plays(s.world.Roles().Named(MortalRole)))
	// a wanderer walks to the other end of its row and back, a second's rest at each end
	units.Define(WandererKind, land, profile,
		comp.Load(func(u unitRow) navigation.MoveOrder { return navigation.Patrol(time.Second, u.to, u.start) }),
		rule.Plays(s.world.Roles().Named(MortalRole)))
}

func (s *arena) cellAt(x, y uint32) cell.ID { c := s.brd.CellIndex(x, y); return c }

func (s *arena) spawnCells() {
	var cells []cell.Entry
	for _, l := range levers {
		for y := stripTop; y <= stripBottom; y++ {
			for x := l.left; x <= l.left+1; x++ {
				cells = append(cells, cell.Entry{Kind: BoardsCell, Cell: s.cellAt(x, y), Group: "trapdoors " + l.name})
			}
		}
	}
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}

func (s *arena) spawnUnits() {
	scoutKind := kind.Named[unitRow](s.world.Kinds(), ScoutKind)
	wandererKind := kind.Named[unitRow](s.world.Kinds(), WandererKind)
	for i := range uint32(3) {
		s.world.Seed(scoutKind.Entry(unitRow{start: s.cellAt(3+2*i, GridHeight-2)}).Told(players.Give{To: s.player.ID}, selection.Allow{}))
	}
	for _, row := range rows {
		s.world.Seed(wandererKind.Entry(unitRow{start: s.cellAt(2, row), to: s.cellAt(GridWidth-3, row)}))
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
