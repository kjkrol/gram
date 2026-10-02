package hooks_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/vision/hooks"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

type huntBody struct{ x, y float64 }

func huntAt(d huntBody) world.Position {
	return world.Position{AABB: plane.NewAABB(geom.NewVec(d.x, d.y), 10, 10)}
}

// huntRun is search with the looking around switched off — a pure chaser.
func huntRun(t *testing.T, hunter huntBody, prey []huntBody, bystanders []huntBody) geom.Vec {
	t.Helper()
	return search(t, 0, hunter, prey, bystanders)
}

// search spawns a predator facing east with prey and bystanders, ticks once, returns its heading.
func search(t *testing.T, lookEvery time.Duration, hunter huntBody, prey []huntBody, bystanders []huntBody) geom.Vec {
	t.Helper()
	east := geom.NewVec(1.0, 0.0)

	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 2000, Height: 2000},
		Entities: world.EntitiesCfg{MaxCount: 16, MinSize: 1, MaxSize: 100},
	})
	v := vision.NewPlugin(w)
	tags := hooks.DefineTags(w.Kinds())
	if err := v.Hook(hooks.Chase(tags, lookEvery)); err != nil {
		t.Fatalf("Hook: %v", err)
	}

	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatalf("world Install: %v", err)
	}
	if err := v.Install(ctx); err != nil {
		t.Fatalf("vision Install: %v", err)
	}

	hunters := kind.Define[huntBody](w.Kinds(), "hunter", kind.Spec{
		comp.Load(huntAt),
		comp.Const(world.Velocity{Dir: east, Value: 1}),
		comp.Const(vision.Sight{Facing: east, Radius: 600}), comp.Const(world.Eye{Angle: 2 * math.Pi / 2.5}), comp.Const(vision.Sighted{}),
		comp.Const(steering.Steering{}), comp.Const(steering.Course{}),
		comp.Tagged(tags.Predator),
	})
	preyKind := kind.Define[huntBody](w.Kinds(), "prey", kind.Spec{comp.Load(huntAt), comp.Const(world.Velocity{}), comp.Tagged(tags.Prey)})
	bystanderKind := kind.Define[huntBody](w.Kinds(), "bystander", kind.Spec{comp.Load(huntAt), comp.Const(world.Velocity{})})

	w.Seed(hunters.Entry(hunter))
	for _, p := range prey {
		w.Seed(preyKind.Entry(p))
	}
	for _, b := range bystanders {
		w.Seed(bystanderKind.Entry(b))
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
			if marks.Slice(cur)[i].Has(tags.Predator) {
				out = got.Vel.Dir
			}
		}
	}
	return out
}

func TestHunt_TurnsTowardsThePreyItSees(t *testing.T) {
	got := huntRun(t, huntBody{x: 500, y: 500}, []huntBody{{x: 800, y: 800}}, nil)

	if want := math.Pi / 4; math.Abs(heading(got)-want) > 1e-6 {
		t.Errorf("heading %.4f rad (%v), want %.4f — straight huntAt the prey", heading(got), got, want)
	}
}

func TestHunt_GoesForTheNearestPrey(t *testing.T) {
	got := huntRun(t, huntBody{x: 500, y: 500}, []huntBody{{x: 900, y: 500}, {x: 600, y: 500}}, nil)

	if math.Abs(heading(got)) > 1e-6 {
		t.Errorf("heading %.4f rad, want 0 — huntAt the near prey, dead east", heading(got))
	}
}

func TestHunt_IgnoresWhatIsNotPrey(t *testing.T) {
	east := geom.NewVec(1.0, 0.0)

	if got := huntRun(t, huntBody{x: 500, y: 500}, nil, []huntBody{{x: 800, y: 800}}); got != east {
		t.Errorf("heading %v, want it untouched (%v) — a bystander is not prey", got, east)
	}
}

func TestHunt_LooksAroundWhenThereIsNobodyToChase(t *testing.T) {
	got := search(t, time.Millisecond, huntBody{x: 500, y: 500}, nil, nil)

	if math.Abs(got.X) > 1e-9 || math.Abs(math.Abs(got.Y)-1) > 1e-9 {
		t.Errorf("heading %v, want a quarter turn off east — straight up or straight down", got)
	}
}

func TestHunt_RunsStraightBetweenLooks(t *testing.T) {
	east := geom.NewVec(1.0, 0.0)

	if got := search(t, time.Hour, huntBody{x: 500, y: 500}, nil, nil); got != east {
		t.Errorf("heading %v one tick into an hour's stretch, want it untouched (%v)", got, east)
	}
}

// Prey in view always wins over the search, however overdue a look is.
func TestHunt_ChasesRatherThanLooksAround(t *testing.T) {
	got := search(t, time.Millisecond, huntBody{x: 500, y: 500}, []huntBody{{x: 800, y: 800}}, nil)

	if want := math.Pi / 4; math.Abs(heading(got)-want) > 1e-6 {
		t.Errorf("heading %.4f rad, want %.4f — huntAt the prey, not off to one side", heading(got), want)
	}
}
