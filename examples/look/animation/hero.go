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
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/world"
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
	pullEvery   = 2 * time.Second
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
	{Frame: beatStand, Span: 2850 * time.Millisecond},
	{Frame: beatBlink, Span: 150 * time.Millisecond},
}

type unitRow struct {
	Start cell.ID    `yaml:"start"`
	Round [4]cell.ID `yaml:"round"`
}

var heroSheet = render.DecodeSheet("hero.png", heroPNG, heroWidth, heroHeight)

// hero is a figure drawn from the sheet in eight ways, each kind of it showing animations of its
// own: a walker on a patrol steps and brakes, an idler is bored by stages, a puller's hands go
// to a lever now and then.
type hero struct{ kind string }

var _ board.Figure = hero{}

// the heroes' states and their role, shared by the three kinds
func (a *arena) defineEffects() {
	fx := a.world.Write().Effects()
	fx.Define(PullingEf, effect.Spec{effect.Lasts(leverSwing)})
	fx.Define(BrakingEf, effect.Spec{})
	fx.Define(StillEf, effect.Spec{})
	fx.Define(BoredEf, effect.Spec{})
	fx.Define(BlinkingEf, effect.Spec{})
	fx.Define(StampingEf, effect.Spec{})
	fx.Define(StampEf, effect.Spec{effect.Lasts(stampSeries)})
	fx.Define(LookingEf, effect.Spec{})
	fx.Define(LookingLongEf, effect.Spec{effect.Lasts(longLook)})
}

// standing he gets bored, moving he wakes, braking he braces
func (a *arena) defineRules() {
	fx := a.world.Write().Effects()
	a.world.Roles().Define(HeroRole,
		act.Branch[motion.Moving]{
			Name: "stands still",
			When: cond.Self.If(motion.Still()).
				And(cond.Self.If(cond.Not(fx.Named(StillEf)))),
			Then: act.Self.Put(fx.Named(StillEf)),
		},
		act.Branch[motion.Moving]{
			Name: "moves on",
			When: cond.Self.If(cond.Not(motion.Still())),
			Then: wakeUp(fx),
		},
		act.Branch[motion.Moving]{
			Name: "brakes",
			When: cond.Self.If(motion.Braking()),
			Then: act.Self.Hold(fx.Named(BrakingEf)),
		},
	)
}

// boredom off, every stage of it
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

func (h hero) Define(b *board.Plugin) {
	w := b.Read(0).World()
	units := board.NewUnits[unitRow](b, board.Shape{Size: heroHeight, Width: heroWidth},
		func(u unitRow) geom.Vec { return b.Read(0).Board().CellCenter(u.Start) })
	profile := steering.Steering{
		MaxSpeed: heroSpeed,
		Accel:    heroSpeed * 2,
		Brake:    heroSpeed * 4,
		V0:       heroSpeed / 2,
	}
	south := comp.Const(world.Velocity{Dir: geom.NewVec(0, 1)}) // facing the player
	plays := ceremony.Plays(w.Roles().Named(HeroRole))
	land := unit.Mover{Domain: cell.Land}
	switch h.kind {
	case WalkerKind:
		profile.TurnRate = 0.08
		round := comp.Load(func(u unitRow) navigation.Round {
			return navigation.NewRound(u.Round[:]...)
		})
		units.Define(h.kind, land, profile, plays, round, patrol())
	case IdlerKind:
		units.Define(h.kind, land, profile, plays, south, boredom(w))
	case PullerKind:
		units.Define(h.kind, land, profile, south, pulls(w))
	}
}

func (h hero) Look(a *world.Atlas, w *world.Plugin) {
	fx := w.Write().Effects()
	a.Add(kind.Named[unitRow](w.Kinds(), h.kind), heroHeight, standing(0)).
		Facing(heroWays, standing).
		Animated(heroWalkFrames, 120*time.Millisecond, pose(heroWalk)).WhileMoving().
		Under(fx.Named(BrakingEf),
			render.LoopedFacing(heroBrakeFrames, 150*time.Millisecond, pose(heroBrake))).
		Under(fx.Named(BlinkingEf), render.TimedFacing(heroBlinkBeat, beat)).
		Under(fx.Named(StampingEf), render.TimedFacing(heroBlinkBeat, beat)).
		Under(fx.Named(StampEf), render.PhasedFacing(heroStampFrames, stamp)).
		Under(fx.Named(LookingEf), render.TimedFacing(heroBlinkBeat, beat)).
		Under(fx.Named(PullingEf), render.PhasedFacing(heroPullFrames, pose(heroPull)))
}

func patrol() plan.Planned {
	return plan.New(PatrolPlan, plan.Steps(
		plan.Loop[any]{
			Name:  "leg",
			When:  cond.Self.If(cond.Always),
			Then:  plan.Self.Do(navigation.Onward{}),
			Until: cond.Self.If(navigation.Arrived{}),
		},
		plan.Loop[any]{
			Name: "rest",
			When: cond.Self.If(cond.Always),
			Then: plan.Self.Wait(time.Second),
		},
	))
}

// the hands on a lever for a swing, every pullEvery
func pulls(w *world.Plugin) plan.Planned {
	pulling := w.Write().Effects().Named(PullingEf)
	return plan.New(PullsPlan, plan.Loop[any]{
		Name: "pull",
		When: cond.Self.If(cond.Always),
		Then: plan.Steps(plan.Self.Put(pulling), plan.Self.Wait(pullEvery)),
	})
}

// still for a while he is bored, and bored he goes round the stages — blinks, stamps a foot now
// and then, turns to look about twice, each 4 seconds or, on a coin, 10 — until he moves
func boredom(w *world.Plugin) plan.Planned {
	fx := w.Write().Effects()
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

// the row of the way a heading looks, degrees, 0 east, against the clock
func heroWay(angleDeg float64) int { return int(math.Round(angleDeg/360*heroWays)) % heroWays }

func standing(angleDeg float64) render.SpriteDrawer {
	return heroSheet.At(heroStand, heroWay(angleDeg))
}

// the frames of the column col and those after it, each the way of angleDeg
func pose(col int) func(angleDeg float64, frame int) render.SpriteDrawer {
	return func(angleDeg float64, frame int) render.SpriteDrawer {
		return heroSheet.At(col+frame, heroWay(angleDeg))
	}
}

func beat(angleDeg float64, frame int) render.SpriteDrawer {
	return heroSheet.At(heroBeatCols[frame], heroWay(angleDeg))
}

func stamp(angleDeg float64, frame int) render.SpriteDrawer {
	return heroSheet.At(heroStamp+frame%2, heroWay(angleDeg))
}
