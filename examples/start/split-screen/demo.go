package main

import (
	"embed"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
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
	MaxEntCount  = 8

	MinimapShare = 0.2
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world      *world.Plugin
	motion     *motion.Plugin
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
				MinSize:  blockSize,
				MaxSize:  blockSize,
			},
			Heights: true,
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayers).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.motion = motion.NewPlugin(a.world)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.collision = collision.NewPlugin(a.world, a.motion)
	a.board = board.NewPlugin(grid, &cell.MultipleOccupancy{}, a.world).
		WithMotion(a.motion, cell.DefaultHiking).
		WithCollision(a.collision)
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.motion, a.selection, a.driving).
		WithCollision(a.collision)
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{})
	a.players = players.NewPlugin(a.world, a.collision,
		a.cameras, a.board, a.selection, a.nav, a.driving)
	a.board.Write().ShowGrid(false)
}

func (a *arena) definePlayers() error {
	a.redPlayer = a.players.Local("red")
	a.bluePlayer = a.players.Local("blue")
	red := driving.Compass{
		Up:    control.KeyW,
		Down:  control.KeyS,
		Left:  control.KeyA,
		Right: control.KeyD,
		In:    camera.Outside,
	}
	redKeys := append(red.Bindings(), a.selection.FollowKey(control.KeyC))
	if err := a.redPlayer.Bind(redKeys...); err != nil {
		return err
	}
	// one keyboard: bound once, not a player each
	gameKeys := append(a.players.GameBindings(), a.world.DefaultBindings()...)
	if err := a.redPlayer.Bind(gameKeys...); err != nil {
		return err
	}
	blue := driving.Compass{
		Up:    control.KeyArrowUp,
		Down:  control.KeyArrowDown,
		Left:  control.KeyArrowLeft,
		Right: control.KeyArrowRight,
		In:    camera.Outside,
	}
	return a.bluePlayer.Bind(append(blue.Bindings(), a.selection.FollowKey(control.KeyEnter))...)
}

func (a *arena) defineFigures() {
	a.board.Tiles(tiles()...)
	a.board.Surfaces(surface{})
	a.board.Figures(block{kind: RedKind, color: redColor}, block{kind: BlueKind, color: blueColor})
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[blockRow](level, board.Sides{
		RedSide:  {players.Give{To: a.redPlayer.ID}, selection.Allow{Selected: true}},
		BlueSide: {players.Give{To: a.bluePlayer.ID}, selection.Allow{Selected: true}},
	})
}

func (a *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.nav.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
