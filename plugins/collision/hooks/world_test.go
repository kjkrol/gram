package hooks_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/uid"
)

// slider is a collider's row: its left edge and how fast it moves along x.
type slider struct{ x, speed float64 }

// colliding runs a world with collision for ticks over two boxes 10 wide at x 100 and 105, the
// rules hooked on the collision: two sensors standing, or, elastic, two boxes closing head-on at 5
// each. It gives the boxes' entities, left first.
func colliding(t *testing.T, elastic bool, ticks int, rules ...plugin.Rule) (left, right uid.UID64) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 2, MinSize: 10, MaxSize: 10},
	})
	c := collision.NewPlugin(w)
	if err := c.Hook(rules...); err != nil {
		t.Fatalf("Hook: %v", err)
	}
	spec := kind.Spec{
		comp.Load(func(b slider) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(b.x, 100), 10, 10)}
		}),
		comp.Load(func(b slider) world.Velocity {
			var v world.Velocity
			v.SetDelta(geom.NewVec(b.speed, 0))
			return v
		}),
		comp.Const(collision.Collider{}),
	}
	rows := []slider{{x: 100}, {x: 105}}
	if elastic {
		spec = append(spec, comp.Const(collision.Physics{Restitution: 1}))
		rows = []slider{{x: 100, speed: 5}, {x: 105, speed: -5}}
	}
	k := kind.Define[slider](w.Kinds(), "slider", spec)
	for _, r := range rows {
		w.Seed(k.Entry(r))
	}
	if err := w.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}
	ecs := collisiontest.Start(t, w, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		var base goke.Comp[world.Base]
		q := si.NewQueryBuilder(&base).Build()
		for q.All(); q.Next(); {
			cur := q.Cursor()
			for i, id := range cur.IDs {
				if base.Slice(cur)[i].Pos.TopLeft.X < 102 {
					left = id
				} else {
					right = id
				}
			}
		}
	}})
	for range ticks {
		ecs.Tick(time.Second / 60)
	}
	return left, right
}
