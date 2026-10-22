package motion_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/internal/motiontest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type thing struct{}

type rig struct {
	w    *world.Plugin
	m    *motion.Plugin
	ecs  *goke.ECS
	all  *goke.Query
	base goke.Comp[world.Base]
}

// a world of 1000 x 1000, its edges as given, with one thing in it
func newRig(
	t *testing.T,
	edges aabbworld.Edges,
	pos world.Position,
	vel world.Velocity,
	own []comp.Comp,
	rules ...act.Rule,
) *rig {
	t.Helper()
	r := &rig{w: world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000, Edges: edges},
		Entities: world.EntitiesCfg{MaxCount: 10, MinSize: 1, MaxSize: 100},
	})}
	r.m = motion.NewPlugin(r.w)
	spec := append(kind.Spec{comp.Const(pos), comp.Const(vel)}, own...)
	kind.Define[thing](r.w.Kinds(), "thing", spec)
	r.w.Write().Seed(kind.Named[thing](r.w.Kinds(), "thing").Entry(thing{}))
	if err := r.w.Populate(); err != nil {
		t.Fatal(err)
	}
	r.ecs = motiontest.Start(t, r.w, r.m, rules, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.all = si.NewQueryBuilder(&r.base).Build()
	}})
	return r
}

func box(x, y, side float64) world.Position {
	return world.Position{AABB: plane.NewAABB(geom.NewVec(x, y), side, side)}
}

func (r *rig) ticks(n int) {
	for range n {
		r.ecs.Tick(time.Second / 60)
	}
}

// the thing's base; the test fails with none
func (r *rig) thing(t *testing.T) world.Base {
	t.Helper()
	for r.all.All(); r.all.Next(); {
		return r.base.Slice(r.all.Cursor())[0]
	}
	t.Fatal("the thing is gone")
	return world.Base{}
}

func living(q *goke.Query) map[uid.UID64]bool {
	found := map[uid.UID64]bool{}
	for q.All(); q.Next(); {
		for _, id := range q.Cursor().IDs {
			found[id] = true
		}
	}
	return found
}
