package main

import (
	"embed"
	"errors"
	"log"
	"slices"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
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
	MaxEntCount  = 64

	tapFor = 160 * time.Millisecond // a key let go sooner turns the hero without a step
)

//go:embed map.yaml
var mapFile embed.FS

type arena struct {
	world     *world.Plugin
	board     *board.Plugin
	driving   *driving.Plugin
	collision *collision.Plugin
	selection *selection.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	player    *players.Player
}

func main() {
	props := game.Props{
		Title: "gram mechanisms demo",
		Description: "Mechanisms: a lever, a plate and a switch. Walk the hero with WSAD. At the " +
			"lever, turned to it, Space pulls it: its trapdoors open; Space again pushes it " +
			"back and they shut. The plate stood on opens the other trapdoors for a while, " +
			"G flips the gate.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(MechanismsStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  leverSize,
				MaxSize:  CellSize,
			},
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Rules(a.defineRoles).
		Commands(a.defineCommands).
		Controls(a.bindKeys).
		Spawn(a.spawn).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.collision = collision.NewPlugin(a.world)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithCollision(a.collision).
		WithLog(log.Default())
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board).WithTap(tapFor)
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.driving)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	walk := driving.Compass{
		Up: control.KeyW, Down: control.KeyS, Left: control.KeyA, Right: control.KeyD,
	}
	view := cameras.Keys{Wheel: true, ZoomIn: control.KeyEqual, ZoomOut: control.KeyMinus}
	return a.player.Bind(slices.Concat(
		a.players.DefaultBindings(),
		a.world.DefaultBindings(),
		a.collision.DefaultBindings(),
		a.board.DefaultBindings(),
		view.Bindings(),
		walk.Bindings(),
	)...)
}

// mortal is everyone's who may fall into the pit: a role shared, so the arena's
func (a *arena) defineRoles() {
	a.world.Roles().Define(MortalRole, act.Branch[unit.Standing]{
		Name: "fall in",
		When: cond.Self.If(unit.Fallen()),
		Then: act.Self.Do(world.Despawn{}),
	})
}

func (a *arena) defineCommands() {
	fx, cmds := a.world.Effects(), a.world.Commands()
	// the lever pulled opens them, pushed back shuts them: one Toggle serves both
	cmds.Define(
		SwitchWestCmd,
		ceremony.Toggle(fx.Named(OpenEf)).
			On(ceremony.Group("west trapdoors")).
			By(ceremony.Named("west lever")),
	)
	cmds.Define(
		OpenEastCmd,
		ceremony.Put(fx.Named(OpenEf)).
			On(ceremony.Group("east trapdoors")).
			By(ceremony.Named("plate")).
			For(2*time.Second),
	)
	cmds.Define(FlipTheGateCmd, ceremony.Toggle(fx.Named(AjarEf)).On(ceremony.Group("gate")))
	cmds.Define(PullCmd, ceremony.Put(fx.Named(ReachingEf)).On(ceremony.Named(HeroName)))
}

type standingRow struct {
	At cell.ID `yaml:"at"`
}

func standingAt(r standingRow) cell.ID { return r.At }

// the tiles the board is laid with and the figures standing on it, in the order they are defined
func (a *arena) defineFigures() {
	a.board.Tiles(grass{}, pit{}, fence{})
	a.board.Figures(lever{}, hero{}, plate{}, trapdoor{}, gate{})
}

func (a *arena) bindKeys() error {
	return a.player.Bind(
		control.Give(
			control.KeyPress{Key: control.KeySpace},
			"Pull, or push back, the lever at hand",
			a.world.Commands().Named(PullCmd),
		),
		control.Give(
			control.KeyPress{Key: control.KeyG},
			"Flip the gate's switch: open, or shut",
			a.world.Commands().Named(FlipTheGateCmd),
		),
	)
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	sides := board.Sides{PlayerSide: {players.Give{To: a.player.ID}}}
	return errors.Join(
		board.Spawn[standingRow](level, nil),
		board.Spawn[unitRow](level, sides),
	)
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
