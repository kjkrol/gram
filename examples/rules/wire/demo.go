package main

import (
	"embed"
	"errors"
	"log"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	cgoke "github.com/kjkrol/ceremony/goke"
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
	EntitySize   = 20
	LeverSize    = 16 // the lever as it was drawn, its box what is drawn

	leverSwing  = 400 * time.Millisecond
	reachFor    = time.Second     // a press waits for a swinging lever this long
	boredAfter  = 5 * time.Second // still so long, the hero is bored
	stageFor    = 5 * time.Second // each stage of boredom
	shortLook   = 4 * time.Second
	longLook    = 10 * time.Second
	stampSeries = 800 * time.Millisecond
	tapFor      = 160 * time.Millisecond
	UnitSpeed   = CellSize * 2
	MaxEntCount = 64
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
		Title: "gram wire demo",
		Description: "Wires: a lever, a plate and a switch. Walk the hero with WSAD. At the " +
			"lever, turned to it, Space pulls it: its trapdoors open; Space again pushes it " +
			"back and they shut. The plate stood on opens the other trapdoors for a while, " +
			"G flips the gate.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(WireStage).
		World(world.Config{
			Space: world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight},
			Entities: world.EntitiesCfg{
				MaxCount: MaxEntCount,
				MinSize:  LeverSize,
				MaxSize:  CellSize,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRoles).
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

func (a *arena) defineEffects() {
	fx := a.world.Effects()
	fx.Define(OpenEf, effect.Spec{effect.Alter(func(f *cell.Floor) { f.Allows = 0 })})
	fx.Define(AjarEf, effect.Spec{effect.Alter(func(w *cell.Wall) { w.Keeps = 0 })})
	fx.Define(ReachingEf, effect.Spec{effect.Lasts(reachFor)})
	fx.Define(PullingEf, effect.Spec{effect.Lasts(leverSwing)})
	fx.Define(InReachEf, effect.Spec{})
	fx.Define(BrakingEf, effect.Spec{})
	fx.Define(StillEf, effect.Spec{})
	fx.Define(BoredEf, effect.Spec{})
	fx.Define(BlinkingEf, effect.Spec{})
	fx.Define(StampingEf, effect.Spec{})
	fx.Define(StampEf, effect.Spec{effect.Lasts(stampSeries)})
	fx.Define(LookingEf, effect.Spec{})
	fx.Define(LookingLongEf, effect.Spec{effect.Lasts(longLook)})
	fx.Define(LeverPulledEf, effect.Spec{})
	fx.Define(LeverPullingEf, effect.Spec{
		effect.Lasts(leverSwing),
		effect.Then(fx.Named(LeverPulledEf)),
	})
	fx.Define(LeverPushingEf, effect.Spec{effect.Lasts(leverSwing)})
}

func (a *arena) defineRoles() {
	roles, fx := a.world.Roles(), a.world.Effects()
	roles.Define(PlateRole, act.Branch[collision.Meeting]{
		Name: "press",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(ceremony.Trigger{}),
	})
	roles.Define(LeverRole)
	atHand := cond.Other.If(roles.Named(LeverRole)).And(cond.Self.If(collision.Facing(math.Pi / 4)))
	// a lever swinging is not pulled again: the hands stay on it as long as the swing
	leverStill := cond.Other.If(cond.Not(fx.Named(LeverPullingEf))).
		And(cond.Other.If(cond.Not(fx.Named(LeverPushingEf))))
	roles.Define(HandyRole,
		act.Branch[collision.Meeting]{
			Name: "a lever at hand",
			When: atHand,
			Then: act.Self.Hold(fx.Named(InReachEf)),
		},
		act.Branch[collision.Meeting]{
			Name: "pull the lever at hand",
			When: cond.Self.If(fx.Named(ReachingEf)).And(atHand).And(leverStill).
				And(cond.Other.If(cond.Not(fx.Named(LeverPulledEf)))),
			Then: act.Steps(
				act.Self.Take(fx.Named(ReachingEf)),
				wakeUp(fx),
				act.Self.Put(fx.Named(PullingEf)),
				act.Other.Put(fx.Named(LeverPullingEf)),
				act.Other.Do(ceremony.Trigger{}),
			),
		},
		act.Branch[collision.Meeting]{
			Name: "push the lever at hand",
			When: cond.Self.If(fx.Named(ReachingEf)).And(atHand).And(leverStill).
				And(cond.Other.If(fx.Named(LeverPulledEf))),
			Then: act.Steps(
				act.Self.Take(fx.Named(ReachingEf)),
				wakeUp(fx),
				act.Self.Put(fx.Named(PullingEf)),
				act.Other.Take(fx.Named(LeverPulledEf)),
				act.Other.Put(fx.Named(LeverPushingEf)),
				act.Other.Do(ceremony.Trigger{}),
			),
		},
	)
	roles.Define(MortalRole, act.Branch[unit.Standing]{
		Name: "fall in",
		When: cond.Self.If(unit.Fallen()),
		Then: act.Self.Do(world.Despawn{}),
	})
	roles.Define(HeroRole,
		act.Branch[world.Moving]{
			Name: "stands still",
			When: cond.Self.If(world.Still()).
				And(cond.Self.If(cond.Not(fx.Named(StillEf)))).
				And(cond.Self.If(cond.Not(fx.Named(PullingEf)))),
			Then: act.Self.Put(fx.Named(StillEf)),
		},
		act.Branch[world.Moving]{
			Name: "moves on",
			When: cond.Self.If(cond.Not(world.Still())),
			Then: wakeUp(fx),
		},
		act.Branch[world.Moving]{
			Name: "brakes",
			When: cond.Self.If(world.Braking()),
			Then: act.Self.Hold(fx.Named(BrakingEf)),
		},
	)
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

func (a *arena) defineCells() {
	kinds := a.board.CellKinds()
	kinds.Define(GrassCell, cell.Kind{Cost: 1, Allows: cell.Land})
	kinds.Define(PitCell, cell.Kind{Cost: 1})
	kinds.Define(FenceCell, cell.Kind{Cost: 1, Solid: true})
}

type unitRow struct {
	Start cell.ID `yaml:"start"`
}

type standingRow struct {
	At cell.ID `yaml:"at"`
}

func (a *arena) defineKinds() {
	brd := a.board.Board()
	roles := a.world.Roles()
	units := board.NewUnits[unitRow](
		a.board,
		board.Shape{Size: heroHeight, Width: heroWidth}, // the box is the figure, pixel for pixel
		func(u unitRow) geom.Vec { return brd.CellCenter(u.Start) },
	)
	profile := steering.Steering{
		MaxSpeed: UnitSpeed,
		Accel:    UnitSpeed * 4, // full speed in a quarter of a second
		Brake:    UnitSpeed * 6,
		TurnRate: 0, // eight ways drawn: the hero turns at once
	}
	units.Define(HeroKind, unit.Mover{Domain: cell.Land}, profile,
		comp.Const(world.Velocity{Dir: geom.NewVec(0, 1)}), // facing the player
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: 1}),
		ceremony.Plays(roles.Named(HandyRole), roles.Named(MortalRole), roles.Named(HeroRole)),
		a.boredom())
	at := func(r standingRow) cell.ID { return r.At }
	cellWide := board.NewEntities(a.board, board.Shape{Size: CellSize}, at)
	cellWide.Define(TrapdoorKind, comp.Const(cell.Floor{Allows: cell.Land}))
	cellWide.Define(GateKind, comp.Const(cell.Wall{Keeps: cell.Land}))
	small := board.NewEntities(a.board, board.Shape{Size: EntitySize}, at)
	small.Define(PlateKind,
		comp.Const(collision.Collider{}), // a sensor: stood on, never pushed
		ceremony.Plays(roles.Named(PlateRole)))
	posts := board.NewEntities(a.board, board.Shape{Size: LeverSize}, at)
	posts.Define(LeverKind,
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: math.Inf(1)}), // a post: the hero walks up to it
		ceremony.Plays(roles.Named(LeverRole)))
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

// boredom off, every stage of it: the hero moves, or pulls
func wakeUp(fx *cgoke.Effects) act.Step {
	return act.Steps(
		act.Self.Take(fx.Named(StillEf)),
		act.Self.Take(fx.Named(BoredEf)),
		act.Self.Take(fx.Named(BlinkingEf)),
		act.Self.Take(fx.Named(StampingEf)),
		act.Self.Take(fx.Named(StampEf)),
		act.Self.Take(fx.Named(LookingEf)),
		act.Self.Take(fx.Named(LookingLongEf)),
	)
}

// the hero's boredom: still for a while he is bored, and bored he goes round the stages — blinks,
// stamps a foot now and then, turns to look about twice, each 4 seconds or, on a coin, 10 —
// until he moves
func (a *arena) boredom() plan.Planned {
	fx := a.world.Effects()
	still, bored, long := fx.Named(StillEf), fx.Named(BoredEf), fx.Named(LookingLongEf)
	always, coin := cond.Self.If(cond.Always), cond.Self.If(cond.Chance(0.5))
	stage := func(e effect.Effect, parts ...plan.Part) plan.Part {
		return plan.Steps(append(append([]plan.Part{plan.Self.Put(e)}, parts...),
			plan.Self.Take(e))...)
	}
	// a long look lasts its time from the coin; the short wait passes under it, then the rest
	look := plan.Steps(
		plan.OneOf(
			act.Branch[any]{When: coin, Then: plan.Self.Do(steering.Turn{Angle: math.Pi / 4})},
			plan.Self.Do(steering.Turn{Angle: -math.Pi / 4}),
		),
		plan.OneOf(act.Branch[any]{When: coin, Then: plan.Self.Put(long)}, plan.Self.Wait(0)),
		plan.Self.Wait(shortLook),
		plan.Loop[any]{Name: "a long look", When: always, Then: plan.Self.Idle(),
			Until: cond.Self.If(cond.Not(long))},
	)
	return plan.New(BoredPlan, plan.OneOf(
		plan.Loop[any]{Name: "bored", When: cond.Self.If(bored), Then: plan.Steps(
			stage(fx.Named(BlinkingEf), plan.Self.Wait(stageFor)),
			stage(fx.Named(StampingEf),
				plan.Self.Put(fx.Named(StampEf)), plan.Self.Wait(4*time.Second),
				plan.Self.Put(fx.Named(StampEf)), plan.Self.Wait(time.Second)),
			stage(fx.Named(LookingEf), look, look),
		)},
		plan.Loop[any]{Name: "still", When: cond.Self.If(still),
			Then: plan.Steps(plan.Self.Wait(boredAfter), plan.Self.Put(bored))},
		plan.Loop[any]{Name: "idle", When: always, Then: plan.Self.Idle()},
	))
}
