package main

import (
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
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
		Title:       "gram pressure plate demo",
		Description: "Pressure plates and trapdoors.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(PressurePlateStage).
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
			Rules(s.defineRules).
			Commands(s.defineCommands).
			Kinds(s.defineCells, s.defineKinds).
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
	kinds := s.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(BoardsCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(PlateCell, cell.Kind{Cost: 1, Allows: cell.Land}, s.world.Roles().Named(PlateRole))
	kinds.Define(PitCell, cell.Kind{Cost: 1})
}

func (s *arena) defineEffects() {
	kinds := s.board.CellKinds()
	s.world.Effects().Define(OpenEf, effect.Spec{
		effect.Lasts(time.Second),
		effect.Alter(func(g *cell.Ground) { g.Kind = kinds.Named(PitCell).Kind() }),
	})
}

func (s *arena) defineRules() {
	s.world.Roles().Define(PlateRole,
		rule.Then[cell.Now]("press", rule.All, rule.If(cell.Now.Stood, rule.Trigger())))
	s.world.Roles().Define(MortalRole,
		rule.Then[unit.Standing](
			"fall in",
			rule.All,
			rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{})),
		))
}

// groups are a plate each and the trapdoors it opens: the plate's column and the first of its
// strip's.
var groups = []struct {
	name        string
	plate, left uint32
}{{"west", 3, 7}, {"east", GridWidth - 4, 15}}

func (s *arena) defineCommands() {
	for _, g := range groups {
		s.world.Commands().Define(openCmd(g.name),
			rule.Cast(s.world.Effects().Named(OpenEf)).
				On(entity.Group("trapdoors "+g.name)).
				By(entity.Named("plate "+g.name)),
		)
	}
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
	land := unit.Mover{Domain: cell.Land}
	units.Define(ScoutKind, land, profile, rule.Plays(s.world.Roles().Named(MortalRole)))
	// a wanderer walks to the other end of its row and back, a second's rest at each end
	units.Define(
		WandererKind,
		land,
		profile,
		comp.Load(func(u unitRow) navigation.MoveOrder {
			return navigation.Patrol(time.Second, u.to, u.start)
		}),
		rule.Plays(s.world.Roles().Named(MortalRole)),
	)
}

// TODO; z pliku
func (s *arena) spawnCells() {
	brd := s.board.Board()
	var cells []cell.Entry
	for _, g := range groups {
		plate := brd.At(g.plate, GridHeight-2)
		cells = append(cells, cell.Entry{Kind: PlateCell, Cell: plate, Name: "plate " + g.name})
		group := "trapdoors " + g.name
		for y := uint32(2); y <= 13; y++ {
			for x := g.left; x <= g.left+1; x++ {
				door := cell.Entry{Kind: BoardsCell, Cell: brd.At(x, y), Group: group}
				cells = append(cells, door)
			}
		}
	}
	s.board.Seed(board.Layout{Default: GrassCell, Cells: cells})
}

// TODO: z pliku
func (s *arena) spawnUnits() {
	brd := s.board.Board()
	scoutKind := kind.Named[unitRow](s.world.Kinds(), ScoutKind)
	wandererKind := kind.Named[unitRow](s.world.Kinds(), WandererKind)
	for i := range uint32(3) {
		s.world.Seed(
			scoutKind.Entry(unitRow{start: brd.At(GridWidth/2-2+2*i, GridHeight-2)}).
				Told(players.Give{To: s.player.ID}, selection.Allow{}),
		)
	}
	for _, row := range []uint32{3, 6, 9, 12} {
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
