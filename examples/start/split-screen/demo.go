package main

import (
	"embed"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
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

//go:embed map.yaml
var mapFile embed.FS

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

func main() {
	props := game.Props{
		Title:       "gram split screen demo",
		Description: "Red: WSAD, blue: arrows.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(SplitScreenStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: WorldWidth, Height: WorldHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  BlockSize,
				MaxSize:  BlockSize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayers).
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
	s.board = board.NewPlugin(grid, &cell.MultipleOccupancy{}, s.world).WithCollision(s.collision)
	s.selection = selection.NewPlugin(s.world)
	s.driving = driving.NewPlugin(s.world, s.selection).WithGround(s.board)
	s.nav = navigation.NewPlugin(s.board, s.world, s.selection, s.driving).
		WithCollision(s.collision)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras, s.board, s.selection, s.nav, s.driving)
}

func (s *arena) definePlayers() error {
	s.redPlayer = s.players.Local("red")
	s.bluePlayer = s.players.Local("blue")
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
	// one keyboard: bound once, not a player each
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

func (s *arena) defineCells() {
	kinds := s.board.CellKinds()
	kinds.Define(FloorCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true})
}

type block struct {
	Start cell.ID `yaml:"start"`
}

func (s *arena) defineKinds() {
	brd := s.board.Board()
	units := board.NewUnits[block](
		s.board,
		board.Shape{Size: BlockSize},
		func(b block) geom.Vec { return brd.CellCenter(b.Start) },
	)
	profile := steering.Steering{
		MaxSpeed: BlockSpeed,
		Accel:    BlockSpeed * 3,
		Brake:    BlockSpeed * 6,
		TurnRate: 0.3,
	}
	units.Define(RedKind, unit.Mover{Domain: cell.Land}, profile)
	units.Define(BlueKind, unit.Mover{Domain: cell.Land}, profile)
}

func (s *arena) spawn() error {
	level, err := s.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[block](level, board.Sides{
		RedSide:  {players.Give{To: s.redPlayer.ID}, selection.Allow{Selected: true}},
		BlueSide: {players.Give{To: s.bluePlayer.ID}, selection.Allow{Selected: true}},
	})
}

func (s *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(s.players.Handle)}
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
