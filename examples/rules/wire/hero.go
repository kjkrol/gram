package main

import (
	_ "embed"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/render"
)

// eight rows of frames 16 wide and 32 tall, the ways east first against the clock; the columns
// standing, four steps, three pulls, a blink, two of a stamp, two of braking
//
//go:embed hero.png
var heroPNG []byte

const (
	heroWidth       = 16
	heroHeight      = 32
	heroWays        = 8
	heroStand       = 0
	heroWalk        = 1
	heroWalkFrames  = 4
	heroPull        = 5
	heroPullFrames  = 3
	heroBlink       = 8
	heroStamp       = 9
	heroStampFrames = 6 // the foot up and down thrice along the effect's course
	heroBrake       = 11
	heroBrakeFrames = 2

	heroSpeed   = CellSize * 2
	reachFor    = time.Second     // a press waits for a swinging lever this long
	boredAfter  = 5 * time.Second // still so long, the hero is bored
	stageFor    = 5 * time.Second // each stage of boredom
	shortLook   = 4 * time.Second
	longLook    = 10 * time.Second
	stampSeries = 800 * time.Millisecond
)

// the frames of a beat: standing, the eyes shut
const (
	beatStand = iota
	beatBlink
)

var heroBeatCols = [...]int{beatStand: heroStand, beatBlink: heroBlink}

// the eyes shut for 150 ms every 3 s
var heroBlinkBeat = []render.Beat{
	beat(beatStand, 2850*time.Millisecond),
	beat(beatBlink, 150*time.Millisecond),
}

func beat(frame int, span time.Duration) render.Beat {
	return render.Beat{Frame: frame, Span: span}
}

type unitRow struct {
	Start cell.ID `yaml:"start"`
}

// hero is the one the player walks: a figure of the board, drawn from its sheet in eight ways,
// stepping, pulling, braking and bored by stages.
type hero struct{ sheet render.Sheet }

var (
	_ board.Figure   = (*hero)(nil)
	_ board.Stateful = (*hero)(nil)
	_ board.Ruled    = (*hero)(nil)
)

func newHero() *hero {
	return &hero{sheet: render.DecodeSheet("hero.png", heroPNG, heroWidth, heroHeight)}
}

func (h *hero) Effects(b *board.Plugin) {
	fx := b.World().Effects()
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
}

// the hero's rules: handy, he uses the lever he touches facing it; standing he gets bored,
// moving he wakes, braking he braces. Mortal is everyone's, the arena's.
func (h *hero) Rules(b *board.Plugin) {
	roles, fx := b.World().Roles(), b.World().Effects()
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

func (h *hero) Define(b *board.Plugin) {
	roles := b.World().Roles()
	units := board.NewUnits[unitRow](b,
		board.Shape{Size: heroHeight, Width: heroWidth}, // the box is the figure, pixel for pixel
		func(u unitRow) geom.Vec { return b.Board().CellCenter(u.Start) })
	profile := steering.Steering{
		MaxSpeed: heroSpeed,
		Accel:    heroSpeed * 4, // full speed in a quarter of a second
		Brake:    heroSpeed * 6,
		TurnRate: 0, // eight ways drawn: the hero turns at once
	}
	units.Define(HeroKind, unit.Mover{Domain: cell.Land}, profile,
		comp.Const(world.Velocity{Dir: geom.NewVec(0, 1)}), // facing the player
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Mass: 1}),
		ceremony.Plays(roles.Named(HandyRole), roles.Named(MortalRole), roles.Named(HeroRole)),
		h.boredom(b.World()))
}

func (h *hero) Look(a *world.Atlas, w *world.Plugin) {
	fx := w.Effects()
	a.Add(kind.Named[unitRow](w.Kinds(), HeroKind), heroHeight, h.standing(0)).
		Facing(heroWays, h.standing).
		Animated(heroWalkFrames, 120*time.Millisecond, h.pose(heroWalk)).WhileMoving().
		Under(fx.Named(BrakingEf),
			render.LoopedFacing(heroBrakeFrames, 150*time.Millisecond, h.pose(heroBrake))).
		Under(fx.Named(BlinkingEf), render.TimedFacing(heroBlinkBeat, h.beat)).
		Under(fx.Named(StampingEf), render.TimedFacing(heroBlinkBeat, h.beat)).
		Under(fx.Named(StampEf), render.PhasedFacing(heroStampFrames, h.stamp)).
		Under(fx.Named(LookingEf), render.TimedFacing(heroBlinkBeat, h.beat)).
		Under(fx.Named(PullingEf), render.PhasedFacing(heroPullFrames, h.pose(heroPull)))
}

// the row of the way a heading looks, degrees, 0 east, against the clock
func heroWay(angleDeg float64) int { return int(math.Round(angleDeg/360*heroWays)) % heroWays }

func (h *hero) standing(angleDeg float64) render.SpriteDrawer {
	return h.sheet.At(heroStand, heroWay(angleDeg))
}

// the frames of the column col and those after it, each the way of angleDeg
func (h *hero) pose(col int) func(angleDeg float64, frame int) render.SpriteDrawer {
	return func(angleDeg float64, frame int) render.SpriteDrawer {
		return h.sheet.At(col+frame, heroWay(angleDeg))
	}
}

func (h *hero) beat(angleDeg float64, frame int) render.SpriteDrawer {
	return h.sheet.At(heroBeatCols[frame], heroWay(angleDeg))
}

func (h *hero) stamp(angleDeg float64, frame int) render.SpriteDrawer {
	return h.sheet.At(heroStamp+frame%2, heroWay(angleDeg))
}

// the hero's boredom: still for a while he is bored, and bored he goes round the stages — blinks,
// stamps a foot now and then, turns to look about twice, each 4 seconds or, on a coin, 10 —
// until he moves
func (h *hero) boredom(w *world.Plugin) plan.Planned {
	fx := w.Effects()
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
