package main

import (
	"embed"
	"log"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/plan"
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
	EntitySize   = 22
	UnitSpeed    = CellSize * 2
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
				MinSize:  EntitySize,
				MaxSize:  EntitySize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Commands(a.defineCommands).
		Kinds(a.defineCells, a.defineKinds).
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
	a.players = players.NewPlugin(a.world, a.collision, a.cameras, a.board, a.selection, a.nav, a.driving)
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

func (a *arena) defineCells() {
	kinds := a.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(WallCell, cell.Kind{Cost: 1, Solid: true})
}

type guardRow struct {
	Start cell.ID    `yaml:"start"`
	Round [2]cell.ID `yaml:"round"`
}

type wandererRow struct {
	Start cell.ID `yaml:"start"`
	To    cell.ID `yaml:"to"`
}

func (a *arena) defineKinds() {
	brd := a.board.Board()
	profile := steering.Steering{
		MaxSpeed: UnitSpeed,
		Accel:    UnitSpeed * 2,
		Brake:    UnitSpeed * 4,
		V0:       UnitSpeed / 2,
		TurnRate: 0.15,
	}
	alarm := a.world.Effects().Named(AlarmEf)
	guard := plan.New(GuardPlan, plan.OneOf(
		plan.Loop[any]{
			Name:    "to the gate",
			When:    cond.On(a.world).If(alarm),
			Then:    plan.Self.Do(navigation.MoveTo{To: ceremony.Named(GateName)}),
			Until:   cond.Self.If(navigation.Arrived{}),
			Timeout: 20 * time.Second,
		},
		plan.Loop[navigation.Blocked]{
			Name: "blocked",
			When: cond.Self.If(navigation.Stuck()),
			Then: plan.Self.Do(navigation.StepAside{}),
		},
		plan.Loop[navigation.Arrived]{
			Name: "rest",
			When: cond.Self.If(cond.Always),
			Then: plan.Self.Wait(2 * time.Second),
		},
		plan.Loop[any]{
			Name:  "patrol",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(navigation.Onward{}),
			Until: cond.Self.If(navigation.Arrived{}),
		},
		plan.Loop[any]{Name: "idle", When: cond.Self.If(cond.Always), Then: plan.Self.Idle()},
	))
	guards := board.NewUnits[guardRow](a.board, board.Shape{Size: EntitySize},
		func(g guardRow) geom.Vec { return brd.CellCenter(g.Start) })
	guards.Define(GuardKind, unit.Mover{Domain: cell.Land}, profile, guard,
		comp.Load(func(g guardRow) navigation.Round {
			return navigation.NewRound(g.Round[:]...)
		}))

	patrol := plan.New(PatrolPlan, plan.Steps(
		plan.Loop[any]{
			Name:  "leg",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(navigation.Onward{}),
			Until: cond.Self.If(navigation.Arrived{}),
		},
		plan.Loop[any]{Name: "rest", When: cond.Self.If(cond.Always), Then: plan.Self.Wait(3 * time.Second)},
	))
	wanderers := board.NewUnits[wandererRow](a.board, board.Shape{Size: EntitySize},
		func(w wandererRow) geom.Vec { return brd.CellCenter(w.Start) })
	wanderers.Define(WandererKind, unit.Mover{Domain: cell.Land}, profile, patrol,
		comp.Load(func(w wandererRow) navigation.Round {
			return navigation.NewRound(w.To, w.Start)
		}))
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
