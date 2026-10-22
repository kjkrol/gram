package main

import (
	"embed"
	"slices"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/effect"
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
	"github.com/kjkrol/gram/plugins/gravity"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS         = 60
	GridWidth   = 48
	GridHeight  = 18
	CellSize    = 16
	MaxEntCount = 8

	pull = 900 // in cells
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world     *world.Plugin
	motion    *motion.Plugin
	board     *board.Plugin
	collision *collision.Plugin
	gravity   *gravity.Plugin
	selection *selection.Plugin
	driving   *driving.Plugin
	cameras   *cameras.Plugin
	players   *players.Plugin
	player    *players.Player
}

func main() {
	props := game.Props{
		Title: "gram platformer demo",
		Description: "A hero in a side view under gravity: A and D or the arrows walk, " +
			"W or the up arrow jumps, G turns gravity over; a hard landing dazes.",
		ScreenWidth:  960,
		ScreenHeight: 540,
		Resizable:    true,
		TargetTPS:    TPS,
	}
	a := &arena{}
	mainStage := stage.New(PlatformerStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: GridWidth * CellSize, Height: GridHeight * CellSize},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  heroSize,
				MaxSize:  heroSize,
			},
			Heights: true,
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayers).
		Effects(a.defineEffects).
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
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.collision = collision.NewPlugin(a.world, a.motion)
	a.gravity = gravity.NewPlugin(a.world, a.motion, gravity.Config{Pull: gravity.Pull{Y: pull}})
	a.board = board.NewPlugin(grid, &cell.MultipleOccupancy{}, a.world).
		WithMotion(a.motion).
		WithCollision(a.collision)
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.motion, a.selection).WithGround(a.board)
	a.cameras = cameras.NewPlugin(a.world, cameras.Config{})
	a.players = players.NewPlugin(
		a.world,
		a.collision, a.cameras,
		a.selection,
		a.driving,
		a.gravity,
	)
}

func (a *arena) definePlayers() error {
	a.player = a.players.Local("player")
	keys := slices.Concat(
		a.players.DefaultBindings(),
		a.world.DefaultBindings(),
		cameras.Keys{Wheel: true, Farther: control.KeyEqual, Nearer: control.KeyMinus}.Bindings(),
	)
	walk := []driving.Compass{
		{Left: control.KeyA, Right: control.KeyD},
		{Left: control.KeyArrowLeft, Right: control.KeyArrowRight},
	}
	for _, w := range walk {
		keys = append(keys, w.Bindings()...)
	}
	return a.player.Bind(append(keys,
		driving.JumpKey(control.KeyW),
		driving.JumpKey(control.KeyArrowUp),
	)...)
}

func (a *arena) defineEffects() {
	a.world.Write().Effects().Define(FlippedEf, effect.Spec{
		effect.Alter(func(p *gravity.Pull) { p.Y = -p.Y }),
	})
}

func (a *arena) defineCommands() {
	flip := ceremony.Toggle(a.world.Write().Effects().Named(FlippedEf)).On(a.gravity)
	a.world.Commands().Define(FlipCmd, flip)
}

func (a *arena) defineFigures() {
	a.board.Tiles(tiles()...)
	a.board.Surfaces(surface{})
	a.board.Figures(hero{})
}

func (a *arena) bindKeys() error {
	return a.player.Bind(control.Give(
		control.KeyPress{Key: control.KeyG},
		"Turn gravity over",
		a.world.Commands().Named(FlipCmd),
	))
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	return board.Spawn[heroRow](level, board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}},
	})
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.motion.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.gravity.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
