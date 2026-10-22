package main

import (
	"embed"
	"log"
	"slices"
	"time"

	"github.com/kjkrol/ceremony"
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
		Title: "gram material demo",
		Description: "Materials of the game's own WGSL: a ward pulsing on the clock, four embers " +
			"circling it drawn out of their own state, breathing standing and streaming driven " +
			"(WSAD the selected). G calms the ward, F douses the selected embers.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(MaterialStage).
		World(world.Config{
			Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{MaxCount: 8, MinSize: emberPx, MaxSize: wardPx},
			Heights:  true,
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Commands(a.defineCommands).
		Controls(a.bindKeys).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.motion = motion.NewPlugin(a.world)
	a.collision = collision.NewPlugin(a.world, a.motion)
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithMotion(a.motion).
		WithCollision(a.collision).
		WithLog(log.Default())
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.motion, a.selection, a.driving).
		WithCollision(a.collision)
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{})
	a.players = players.NewPlugin(a.world, a.collision,
		a.cameras, a.board, a.selection, a.nav, a.driving)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	view := cameras.Keys{
		Wheel:   true,
		Drag:    true,
		Edge:    true,
		Farther: control.KeyEqual,
		Nearer:  control.KeyMinus,
	}
	drive := driving.Tank{
		Ahead: control.KeyW,
		Back:  control.KeyS,
		Left:  control.KeyA,
		Right: control.KeyD,
		In:    camera.Outside,
	}
	return a.player.Bind(slices.Concat(
		a.players.DefaultBindings(),
		a.world.DefaultBindings(),
		view.Bindings(),
		a.board.DefaultBindings(),
		a.selection.DefaultBindings(),
		a.nav.DefaultBindings(),
		a.driving.DefaultBindings(),
		drive.Bindings(),
	)...)
}

func (a *arena) defineCommands() {
	a.world.Commands().Define(CalmCmd,
		ceremony.Put(a.world.Write().Effects().Named(CalmEf)).On(ceremony.Named(WardName)))
	a.world.Commands().Define(DouseCmd,
		ceremony.Put(a.world.Write().Effects().Named(DousedEf)).On(a.selection.Selected()))
}

func (a *arena) defineFigures() {
	a.board.Tiles(grass{})
	a.board.Surfaces(surface{})
	a.board.Figures(ward{}, ember{})
}

func (a *arena) bindKeys() error {
	return a.player.Bind(
		control.Give(control.KeyPress{Key: control.KeyG}, "Calm the ward for a while",
			a.world.Commands().Named(CalmCmd)),
		control.Give(control.KeyPress{Key: control.KeyF}, "Douse the selected embers for a while",
			a.world.Commands().Named(DouseCmd)),
	)
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	if err := board.Spawn[wardRow](level, nil); err != nil {
		return err
	}
	return board.Spawn[unitRow](level, board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{}},
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
