package main

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
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
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 40
	GridHeight   = 30
	CellSize     = 32
	WorldWidth   = GridWidth * CellSize
	WorldHeight  = GridHeight * CellSize
	ScreenWidth  = 1280
	ScreenHeight = 720
	BlockSize    = 24
	BlockSpeed   = CellSize * 5
	MaxEntCount  = 8

	MinimapShare = 0.2
)

type arena struct {
	world      *world.Plugin
	collision  *collision.Plugin
	board      *board.Plugin
	selection  *selection.Plugin
	nav        *navigation.Plugin
	driving    *driving.Plugin
	cameras    *cameras.Plugin
	players    *players.Plugin
	redPlayer  *players.Player
	bluePlayer *players.Player
}

func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram split screen demo",
		Description: "Red: WSAD, blue: arrows.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(SplitScreenStage).
			World(world.Config{
				Space: world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
				Entities: world.EntitiesCfg{
					MaxCount: MaxEntCount,
					MinSize:  BlockSize,
					MaxSize:  BlockSize,
				},
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayers).
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
	s.board = board.NewPlugin(grid, &cell.MultipleOccupancy{}, s.world).WithCollision(s.collision)
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving)
}

func (s *arena) definePlayers() {
	s.redPlayer = s.players.Local("red")
	s.bluePlayer = s.players.Local("blue")
}

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(FloorCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true})
}

func (s *arena) bindKeys() error {
	red := driving.Compass{
		Up:    control.KeyW,
		Down:  control.KeyS,
		Left:  control.KeyA,
		Right: control.KeyD,
		In:    camera.Outside,
	}
	redKeys := append(red.Bindings(), s.selection.FollowKey(control.KeyC))
	if err := s.redPlayer.Bind(redKeys...); err != nil {
		return err
	}
	// the keyboard is one: the game's keys and the world's (Space pauses) are bound once
	gameKeys := append(players.GameBindings(), s.world.DefaultBindings()...)
	if err := s.redPlayer.Bind(gameKeys...); err != nil {
		return err
	}
	blue := driving.Compass{
		Up:    control.KeyArrowUp,
		Down:  control.KeyArrowDown,
		Left:  control.KeyArrowLeft,
		Right: control.KeyArrowRight,
		In:    camera.Outside,
	}
	return s.bluePlayer.Bind(append(blue.Bindings(), s.selection.FollowKey(control.KeyEnter))...)
}

// TODO: definicja z pliku !
type block struct{ start cell.ID }

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[block](
		s.board,
		board.Shape{Size: BlockSize},
		func(b block) geom.Vec { return brd.CellCenter(b.start) },
	)
	profile := steering.Steering{
		MaxSpeed: BlockSpeed,
		Accel:    BlockSpeed * 3,
		Brake:    BlockSpeed * 6,
		TurnRate: 0.3,
	}
	// each block is its player's: it takes that player's hand alone
	units.Define(RedKind, unit.Mover{Domain: cell.Land}, profile)
	units.Define(BlueKind, unit.Mover{Domain: cell.Land}, profile)
}

func (s *arena) spawnCells() {
	cellAt := s.board.Board().At
	var cells []cell.Entry
	wall := func(x, y uint32) {
		cells = append(cells, cell.Entry{Kind: WallCell, Cell: cellAt(x, y)})
	}
	for x := uint32(0); x < GridWidth; x++ {
		wall(x, 0)
		wall(x, GridHeight-1)
	}
	for y := uint32(1); y < GridHeight-1; y++ {
		wall(0, y)
		wall(GridWidth-1, y)
	}
	for y := uint32(6); y < 24; y++ {
		wall(20, y)
	}
	for x := uint32(8); x < 16; x++ {
		wall(x, 10)
		wall(x+16, 20)
	}
	for _, p := range [][2]uint32{{6, 20}, {10, 24}, {30, 6}, {34, 12}, {26, 26}, {14, 4}} {
		wall(p[0], p[1])
		wall(p[0]+1, p[1])
		wall(p[0], p[1]+1)
		wall(p[0]+1, p[1]+1)
	}
	s.board.Seed(board.Layout{Default: FloorCell, Cells: cells})
}

func (s *arena) spawnUnits() {
	redKind := kind.Named[block](s.world.Kinds(), RedKind)
	blueKind := kind.Named[block](s.world.Kinds(), BlueKind)
	brd := s.board.Board()
	s.world.Seed(
		redKind.Entry(block{start: brd.At(3, 3)}).
			Named(RedBlock).
			Told(players.Give{To: s.redPlayer.ID}, selection.Allow{Selected: true}),
		blueKind.Entry(block{start: brd.At(GridWidth-4, GridHeight-4)}).
			Named(BlueBlock).
			Told(players.Give{To: s.bluePlayer.ID}, selection.Allow{Selected: true}),
	)
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

func (s *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(s.players.Handle)}
}
