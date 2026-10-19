package main

import (
	"embed"
	"errors"
	"slices"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	GridWidth    = 24
	GridHeight   = 16
	CellSize     = 32
	ScreenWidth  = GridWidth * CellSize
	ScreenHeight = GridHeight * CellSize
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world     *world.Plugin
	motion    *motion.Plugin
	light     *light.Plugin
	board     *board.Plugin
	nav       *navigation.Plugin
	driving   *driving.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	telemetry *telemetry.Plugin
	player    *players.Player
}

func main() {
	props := game.Props{
		Title: "gram light demo",
		Description: "A flat board under a sun of the game's own, circling in a day and a half a " +
			"minute: a tower's shadow baked into the ground, walkers casting patches, a hero " +
			"(WSAD) casting its silhouette, a lantern lighting the night and casting their " +
			"shadows by it, lava that takes no shade. Shift+L the lamps' shadows, H coarser " +
			"shadows.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(LookLightStage).
		World(world.Config{
			Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{MaxCount: 8, MinSize: lanternPx, MaxSize: towerPx},
			Heights:  true,
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Shows(MainScene, telemetry.SceneName).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.motion = motion.NewPlugin(a.world)
	a.light = light.NewPlugin(a.world, light.Config{LampShadows: 1})
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.collision = collision.NewPlugin(a.world, a.motion)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithMotion(a.motion).
		WithCollision(a.collision)
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.motion, a.selection, a.driving).
		WithCollision(a.collision)
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{})
	a.telemetry = telemetry.NewPlugin(a.world).With(a.light.Reporter())
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.selection, a.nav,
		a.driving, a.light)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	walk := driving.Compass{
		Up: control.KeyW, Down: control.KeyS, Left: control.KeyA, Right: control.KeyD,
	}
	view := cameras.Keys{
		Wheel:   true,
		ZoomIn:  control.KeyEqual,
		ZoomOut: control.KeyMinus,
	}
	return a.player.Bind(slices.Concat(
		a.players.DefaultBindings(),
		a.world.DefaultBindings(),
		a.collision.DefaultBindings(),
		a.board.DefaultBindings(),
		a.selection.DefaultBindings(),
		a.nav.DefaultBindings(),
		a.light.DefaultBindings(),
		view.Bindings(),
		walk.Bindings(),
	)...)
}

func (a *arena) defineFigures() {
	a.board.Tiles(grass{}, lava{})
	a.board.Surfaces(surface{})
	a.board.Figures(tower{}, lantern{}, walker{}, hero{})
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return errors.Join(
		board.Spawn[standingRow](level, nil),
		board.Spawn[heroRow](level, board.Sides{
			PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{Selected: true}},
		}),
		board.Spawn[walkerRow](level, board.Sides{
			PatrolSide: {players.Give{To: a.player.ID}, selection.Allow{}},
		}),
	)
}

func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.light.WithDistant(sunAt(a.world.Clock().Time())) // the game's own sun, of the moment
	a.light.RunPlan(ctx, d)
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
