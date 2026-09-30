package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
)

type layered struct {
	x      float64
	layers world.Layers
}

// layersRun ticks two overlapping boxes on the given layers once and reports whether they met and
// how far apart their left edges ended.
func layersRun(t *testing.T, a, b world.Layers) (met bool, gap float64) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	c := collision.NewPlugin(w)
	if err := c.Hook(act.Trigger[collision.Meeting]("hook").Runs(func(plugin.Tick, collision.Meeting) { met = true })); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Install(ctx); err != nil {
		t.Fatal(err)
	}
	boxes := kind.Define[layered](w.Kinds(), "box", kind.Spec{
		comp.Load(func(b layered) world.Position { return posAt(b.x, 500, 10, 10) }),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
		comp.Load(func(b layered) world.Layers { return b.layers }),
		comp.Const(collision.Physics{}),
	})
	w.Seed(boxes.Entry(layered{x: 100, layers: a}), boxes.Entry(layered{x: 104, layers: b}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
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

	var lefts []float64
	for q.All(); q.Next(); {
		for _, b := range base.Slice(q.Cursor()) {
			lefts = append(lefts, b.Pos.TopLeft.X)
		}
	}
	return met, max(lefts[0], lefts[1]) - min(lefts[0], lefts[1])
}

func TestLayers_TouchOnlyWhereTheyShareABit(t *testing.T) {
	cases := map[string]struct {
		a, b  world.Layers
		touch bool
	}{
		"same layer":          {1, 1, true},
		"disjoint layers":     {1, 4, false},
		"overlapping masks":   {3, 6, true},
		"zero meets anything": {0, 4, true},
		"both zero":           {0, 0, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			met, gap := layersRun(t, tc.a, tc.b)
			if met != tc.touch {
				t.Errorf("met = %v, want %v", met, tc.touch)
			}
			if pushedApart := gap >= 10; pushedApart != tc.touch {
				t.Errorf("gap after the tick = %v, want pushed apart = %v", gap, tc.touch)
			}
		})
	}
}
