package hooks_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/vision/hooks"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/steering"
)

// fleeBody is a test entity: where it is, which way it is going (a zero dir is
// something standing still), and whether it is marked as a Threat.
type fleeBody struct {
	x, y  float64
	dir   geom.Vec
	scary bool
}

func fleeAt(d fleeBody) world.Position {
	return world.Position{AABB: plane.NewAABB(geom.NewVec(d.x, d.y), 10, 10)}
}

// fleeRun spawns a skittish runner and the given threats, ticks once, returns the runner's heading.
func fleeRun(t *testing.T, runner fleeBody, facing geom.Vec, threats ...fleeBody) geom.Vec {
	t.Helper()
	return fleeRunWith(t, nil, runner, facing, threats...)
}

func fleeRunWith(t *testing.T, tune func(*hooks.Flee), runner fleeBody, facing geom.Vec, threats ...fleeBody) geom.Vec {
	t.Helper()

	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 2000, Height: 2000},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 1, MaxSize: 100},
	})
	v := vision.NewPlugin(w)
	tags := hooks.DefineTags(w.Kinds())
	avoid := hooks.NewFlee(tags)
	if tune != nil {
		tune(avoid)
	}
	if err := v.Hook(avoid.Rule()); err != nil {
		t.Fatalf("Hook: %v", err)
	}

	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatalf("world Install: %v", err)
	}
	if err := v.Install(ctx); err != nil {
		t.Fatalf("vision Install: %v", err)
	}

	runners := kind.Define[fleeBody](w.Kinds(), "runner", kind.Spec{
		comp.Load(fleeAt),
		comp.Const(world.Velocity{Dir: facing, Value: 1}),
		comp.Const(vision.Sight{Facing: facing, Radius: 600}), comp.Const(world.Eye{Angle: 2 * math.Pi / 2.5}), comp.Const(vision.Sighted{}),
		comp.Const(steering.Steering{}), comp.Const(steering.Course{}),
		comp.Tagged(tags.Skittish),
	})
	moving := func(d fleeBody) world.Velocity {
		if d.dir == (geom.Vec{}) {
			return world.Velocity{}
		}
		return world.Velocity{Dir: d.dir, Value: 1}
	}
	harmless := kind.Define[fleeBody](w.Kinds(), "threat", kind.Spec{comp.Load(fleeAt), comp.Load(moving)})
	predators := kind.Define[fleeBody](w.Kinds(), "predator", kind.Spec{comp.Load(fleeAt), comp.Load(moving), comp.Tagged(tags.Threat)})

	w.Seed(runners.Entry(runner))
	for _, th := range threats {
		of := harmless
		if th.scary {
			of = predators
		}
		w.Seed(of.Entry(th))
	}
	if err := w.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	var base goke.Comp[world.Base]
	var marks goke.Comp[tag.Tags[hooks.Family]]
	var query *goke.Query
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		query = si.NewQueryBuilder(&base, &marks).Build()
	}})
	ctx.ecs.Setup(systems...)

	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { v.RunPlan(rc, d); w.RunPlan(rc, d); w.Clock().Replay(rc, d) })
	ctx.ecs.Tick(time.Second / 60)

	var out geom.Vec
	query.All()
	for query.Next() {
		cur := query.Cursor()
		for i, got := range base.Slice(cur) {
			if marks.Slice(cur)[i].Has(tags.Skittish) {
				out = got.Vel.Dir
			}
		}
	}
	return out
}

func TestFlee_TurnsAwayFromWhatItSees(t *testing.T) {
	east := geom.NewVec(1.0, 0.0)
	got := fleeRun(t, fleeBody{x: 500, y: 500}, east, fleeBody{x: 800, y: 500})

	if math.Abs(heading(got)-math.Pi) > 1e-6 {
		t.Errorf("heading %.4f rad (%v), want pi — straight away from the threat", heading(got), got)
	}
}

// Nothing in view, nothing to decide: the heading is left alone.
func TestFlee_LeavesTheHeadingAloneWithNothingInSight(t *testing.T) {
	east := geom.NewVec(1.0, 0.0)
	if got := fleeRun(t, fleeBody{x: 500, y: 500}, east); got != east {
		t.Errorf("heading %v with an empty cone, want it untouched (%v)", got, east)
	}
}

func TestFlee_CombinesEveryThreatIntoOneTurn(t *testing.T) {
	east := geom.NewVec(1.0, 0.0)
	got := fleeRun(t, fleeBody{x: 500, y: 500}, east,
		fleeBody{x: 800, y: 300},
		fleeBody{x: 800, y: 700},
	)

	if math.Abs(heading(got)-math.Pi) > 0.05 {
		t.Errorf("heading %.4f rad, want about pi — the two pushes should cancel sideways", heading(got))
	}
}

func TestFlee_SwitchedOffLeavesTheHeadingAlone(t *testing.T) {
	east := geom.NewVec(1.0, 0.0)
	got := fleeRunWith(t, func(b *hooks.Flee) { b.SetEnabled(false) },
		fleeBody{x: 500, y: 500}, east, fleeBody{x: 800, y: 500})

	if got != east {
		t.Errorf("heading %v with the behavior off, want it untouched (%v)", got, east)
	}
}

func TestFlee_IgnoresWhatItMerelyPassesBy(t *testing.T) {
	east := geom.NewVec(1.0, 0.0)
	if got := fleeRun(t, fleeBody{x: 500, y: 500}, east, fleeBody{x: 600, y: 714}); got != east {
		t.Errorf("heading %v, want it untouched (%v) — nothing here is on a collision course", got, east)
	}
}

func TestFlee_TurnsAwayFromWhatIsComingAtIt(t *testing.T) {
	east := geom.NewVec(1.0, 0.0)
	towards := geom.NewVec(100, 214)
	closing := geom.NewVec(-towards.X, -towards.Y)
	norm := math.Hypot(closing.X, closing.Y)
	closing = geom.NewVec(closing.X/norm, closing.Y/norm)

	got := fleeRun(t, fleeBody{x: 500, y: 500}, east, fleeBody{x: 600, y: 714, dir: closing})

	if math.Abs(heading(got)-heading(closing)) > 1e-6 {
		t.Errorf("heading %.4f rad, want %.4f — straight away from what is closing in", heading(got), heading(closing))
	}
}

func TestFlee_AlwaysRunsFromAThreatInSight(t *testing.T) {
	east := geom.NewVec(1.0, 0.0)
	away := geom.NewVec(-100, -214)
	norm := math.Hypot(away.X, away.Y)
	away = geom.NewVec(away.X/norm, away.Y/norm)

	got := fleeRun(t, fleeBody{x: 500, y: 500}, east, fleeBody{x: 600, y: 714, scary: true})

	if math.Abs(heading(got)-heading(away)) > 1e-6 {
		t.Errorf("heading %.4f rad, want %.4f — straight away from the threat", heading(got), heading(away))
	}
}

func TestFlee_AThreatOutweighsEverythingElseInSight(t *testing.T) {
	east := geom.NewVec(1.0, 0.0)
	away := geom.NewVec(-100, -214)
	norm := math.Hypot(away.X, away.Y)
	away = geom.NewVec(away.X/norm, away.Y/norm)

	got := fleeRun(t, fleeBody{x: 500, y: 500}, east,
		fleeBody{x: 600, y: 714, scary: true},
		fleeBody{x: 560, y: 500},
	)

	if math.Abs(heading(got)-heading(away)) > 1e-6 {
		t.Errorf("heading %.4f rad, want %.4f — straight away from the threat, the neighbour ignored", heading(got), heading(away))
	}
}
