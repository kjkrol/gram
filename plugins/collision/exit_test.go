package collision_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

type pushedOut struct {
	x    float64
	wall bool
}

func TestCollision_ABoxPushedThroughAnOpenEdgeIsReportedToTheWorld(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000, Edges: aabbworld.OpenX},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	var orders heards
	if err := w.Carry(&orders); err != nil {
		t.Fatal(err)
	}
	if err := w.Hook(rule.On("left", rule.All, func(m *rule.Moment[world.Leaving]) rule.Step { return m.Order(heard{}) })); err != nil {
		t.Fatal(err)
	}

	c := collision.NewPlugin(w)
	boxes := kind.Define[pushedOut](w.Kinds(), "box", kind.Spec{
		comp.Load(func(b pushedOut) world.Position { return posAt(b.x, 500, 10, 10) }),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
		comp.Load(func(b pushedOut) collision.Physics {
			if b.wall {
				return collision.Physics{Mass: math.Inf(1)}
			}
			return collision.Physics{}
		}),
	})
	w.Seed(boxes.Entry(pushedOut{x: 0, wall: true}), boxes.Entry(pushedOut{x: -9}))
	if err := w.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	ecs := collisiontest.Start(t, w, c)
	for range 3 {
		ecs.Tick(time.Second / 60)
	}

	var left []uid.UID64
	for _, h := range orders.given() {
		left = append(left, h.Entity)
	}
	if len(left) == 0 {
		t.Fatal("no Leaving heard, want the box the wall pushed out")
	}
	for _, id := range left {
		if id != left[0] {
			t.Fatalf("heard %v, want one and the same box every tick it is out", left)
		}
	}
}
