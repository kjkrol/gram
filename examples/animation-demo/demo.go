package main

import (
	"log"
	"time"

	"github.com/kjkrol/gram/plugins/cameras"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
	EntitySize   = 24
	UnitSpeed    = CellSize * 2

	// gaitPeriod is how long one frame of the gait is shown, in game time.
	gaitPeriod = 160 * time.Millisecond
)

type unitRow struct {
	start cell.ID
	round [4]cell.ID
}

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

// NewDemo is the animation demo, defined on the arena s: its window and its one Stage, a section at
// a time.
func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram animation demo",
		Description: "A sprite drawn frame after frame; Space freezes the gait.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(AnimationStage).
			World(world.Config{
				Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
				Entities: world.EntitiesCfg{MaxCount: 8, MinSize: EntitySize, MaxSize: EntitySize},
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayer).
			Kinds(s.defineCells, s.defineKinds).
			// TODO: dlaczego tu brak restore ?
			Spawn(s.spawnCells, s.spawnUnits).
			Scenes(s.defineScenes).
			Update(s.update))
}

// makePlugins makes the Stage's plugins over its world w — anew each time the Stage is entered —
// each telling the world of itself as it is made: the Stage uses them in that order.
func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	s.collision = collision.NewPlugin(s.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	s.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, s.world).WithCollision(s.collision).WithLog(log.Default())
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
	s.board.CellKinds().Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
}

func (s *arena) defineKinds() {
	brd := s.board.Res.Logic.Board
	units := board.NewUnits[unitRow](s.board, board.Shape{Size: EntitySize}, func(u unitRow) geom.Vec { return brd.CellCenter(u.start) })
	profile := steering.Steering{MaxSpeed: UnitSpeed, Accel: UnitSpeed * 2, Brake: UnitSpeed * 4, V0: UnitSpeed / 2, TurnRate: 0.08}
	round := comp.Load(func(u unitRow) navigation.MoveOrder {
		return navigation.Patrol(time.Second, u.round[0], u.round[1], u.round[2], u.round[3])
	})
	units.Define(BugKind, unit.Mover{Domain: cell.Land}, profile, round)
}

func (s *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(s.players.Handle)}
}

func (s *arena) spawnCells() {
	s.board.Seed(board.Layout{Default: GrassCell})
}

func (s *arena) spawnUnits() {
	brd := s.board.Res.Logic.Board
	bugKind := kind.Named[unitRow](s.world.Kinds(), BugKind)
	player := []any{players.Give{To: s.player.ID}, selection.Allow{}}
	corners := [4]cell.ID{
		brd.CellIndex(4, 4), brd.CellIndex(GridWidth-5, 4),
		brd.CellIndex(GridWidth-5, GridHeight-5), brd.CellIndex(4, GridHeight-5),
	}
	for i := range corners {
		var round [4]cell.ID
		for j := range round {
			round[j] = corners[(i+1+j)%4]
		}
		s.world.Seed(bugKind.Entry(unitRow{start: corners[i], round: round}).Told(player...))
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
