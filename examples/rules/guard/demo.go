package main

import (
	"embed"
	"log"
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
	unitSize     = 22
	unitSpeed    = CellSize * 2
	MaxEntCount  = 16
)

//go:embed map.yaml
var mapFile embed.FS

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

func main() {
	props := game.Props{
		Title: "gram guard demo",
		Description: "Guards on a plan: they patrol the yard, step aside for whoever blocks them " +
			"long enough, rest where they arrive, and run to the gate while the alarm (A) is on.",
		ScreenWidth:  ScreenWidth,
		ScreenHeight: ScreenHeight,
		Resizable:    true,
		TargetTPS:    TPS,
	}
	a := &arena{}
	mainStage := stage.New(GuardStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  unitSize,
				MaxSize:  unitSize,
			},
			Heights: true,
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
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
	grid := grid.DefaultGrids{}.Square(GridWidth, GridHeight, CellSize)
	a.collision = collision.NewPlugin(a.world)
	a.board = board.NewPlugin(grid, &cell.SingleOccupancy{}, a.world).
		WithCollision(a.collision).
		WithLog(log.Default())
	a.selection = selection.NewPlugin(a.world)
	a.driving = driving.NewPlugin(a.world, a.selection).WithGround(a.board)
	a.nav = navigation.NewPlugin(a.board, a.world, a.selection, a.driving).
		WithCollision(a.collision)
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.collision,
		a.cameras, a.board, a.selection, a.nav, a.driving)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineEffects() {
	a.world.Effects().Define(AlarmEf, effect.Spec{})
}

func (a *arena) defineCommands() {
	a.world.Commands().Define(AlarmCmd,
		ceremony.Toggle(a.world.Effects().Named(AlarmEf)).On(ceremony.World))
}

var profile = steering.Steering{
	MaxSpeed: unitSpeed,
	Accel:    unitSpeed * 2,
	Brake:    unitSpeed * 4,
	V0:       unitSpeed / 2,
	TurnRate: 0.15,
}

func (a *arena) defineFigures() {
	a.board.Tiles(grass{}, wall{})
	a.board.Surfaces(surface{})
	a.board.Figures(guard{}, wanderer{})
}

func (a *arena) bindKeys() error {
	return a.player.Bind(control.Give(
		control.KeyPress{Key: control.KeyA},
		"Sound the alarm, or call it off",
		a.world.Commands().Named(AlarmCmd),
	))
}

func (a *arena) spawn() error {
	level, err := a.board.Lay(mapFile, "map.yaml")
	if err != nil {
		return err
	}
	sides := board.Sides{
		PlayerSide: {players.Give{To: a.player.ID}, selection.Allow{Selected: true}},
	}
	if err := board.Spawn[guardRow](level, sides); err != nil {
		return err
	}
	return board.Spawn[wandererRow](level, sides)
}

func (a *arena) defineScenes() []game.Scene {
	main := &mainScene{arena: a}
	return []game.Scene{ui.NewScene(MainScene, main.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.board.RunPlan(ctx, d)
	a.nav.RunPlan(ctx, d)
	a.driving.RunPlan(ctx, d)
	a.selection.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}
