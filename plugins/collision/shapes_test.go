package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
)

type shaped struct{ x float64 }

// shapesRun ticks two overlapping elastic boxes once under test, returning Meetings and left edges.
func shapesRun(t *testing.T, test collision.ShapeTest) (meetings []collision.Meeting, lefts []float64) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	c := collision.NewPlugin(w)
	if test != nil {
		c.WithShapeTest(test)
	}
	if err := c.Hook(host.Pair(tag.Any, tag.Any, func(_ plugin.Tick, m collision.Meeting) {
		meetings = append(meetings, m)
	})); err != nil {
		t.Fatalf("Hook: %v", err)
	}
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatalf("world Install: %v", err)
	}
	if err := c.Install(ctx); err != nil {
		t.Fatalf("collision Install: %v", err)
	}

	boxes := kind.Define[shaped](w.Kinds(), "box", kind.Spec{
		comp.Load(func(b shaped) world.Position { return posAt(b.x, 500, 10, 10) }),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
	})
	w.Seed(boxes.Entry(shaped{x: 100}), boxes.Entry(shaped{x: 104}))
	if err := w.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	var base goke.Comp[world.Base]
	var q *goke.Query
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base).Build() }})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		c.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	})
	ctx.ecs.Tick(time.Second / 60)

	for q.All(); q.Next(); {
		for _, b := range base.Slice(q.Cursor()) {
			lefts = append(lefts, b.Pos.TopLeft.X)
		}
	}
	return meetings, lefts
}

func TestShapeTest_BoxesTouchIsWhatAnUnsetTestAmountsTo(t *testing.T) {
	unset, _ := shapesRun(t, nil)
	explicit, lefts := shapesRun(t, collision.BoxesTouch)
	if len(unset) != 1 || len(explicit) != 1 {
		t.Fatalf("%d and %d meetings, want the one contact either way", len(unset), len(explicit))
	}
	if lefts[0] >= 100 || lefts[1] <= 104 {
		t.Errorf("boxes at %v, want them pushed apart", lefts)
	}
}

func TestShapeTest_ARefusalMeansNoContactAndNoPush(t *testing.T) {
	asked := 0
	meetings, lefts := shapesRun(t, func(_ plugin.Tick, a, b collision.Contactee, pen geom.Vec) (geom.Vec, bool) {
		asked++
		if a.Base == nil || b.Base == nil || a.ID == b.ID {
			t.Errorf("shape test handed %+v and %+v, want two distinct entities with their Base", a, b)
		}
		return pen, false
	})
	if asked != 1 {
		t.Errorf("shape test asked %d times, want once for the one overlapping pair", asked)
	}
	if len(meetings) != 0 {
		t.Errorf("%d meetings reported for a pair the shapes do not touch in, want none", len(meetings))
	}
	if lefts[0] != 100 || lefts[1] != 104 {
		t.Errorf("boxes at %v, want them left where they were", lefts)
	}
}
