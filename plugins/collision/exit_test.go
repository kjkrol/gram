package collision_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
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

// pushedOutWorld is a world open on x with a wall at its left edge and a box the wall overlaps from
// past the edge, a Leaving rule giving a heard when hooked; it reports the ECS, the heards and
// a query over every box.
func pushedOutWorld(t *testing.T, hooked bool) (*goke.ECS, *heards, *goke.Query, *goke.Comp[world.Base]) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000, Edges: aabbworld.OpenX},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	orders := &heards{}
	if err := w.Carry(orders); err != nil {
		t.Fatal(err)
	}
	if hooked {
		if err := w.Hook(rule.Then[world.Leaving]("left", rule.All, rule.Order(heard{}))); err != nil {
			t.Fatal(err)
		}
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
	base := &goke.Comp[world.Base]{}
	var q *goke.Query
	ecs := collisiontest.Start(t, w, c, goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(base).Build() }})
	return ecs, orders, q, base
}

// The world hears of the box the wall pushed out every tick it is out, one and the same box, and
// it stands where the push left it: wholly past the edge.
func TestCollision_ABoxPushedThroughAnOpenEdgeIsReportedToTheWorld(t *testing.T) {
	ecs, orders, q, base := pushedOutWorld(t, true)
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
	for q.All(); q.Next(); {
		cur := q.Cursor()
		for i, b := range base.Slice(cur) {
			if cur.IDs[i] == left[0] && b.Pos.AABB.BottomRight.X > 0 {
				t.Errorf("the box left at %v, want it wholly past the edge at x=0", b.Pos.AABB.AABB)
			}
		}
	}
}

// With no Leaving rule hooked the world despawns the box the wall pushed out; the wall stays.
func TestCollision_ABoxPushedThroughAnOpenEdgeIsDespawned(t *testing.T) {
	ecs, _, q, base := pushedOutWorld(t, false)
	for range 3 {
		ecs.Tick(time.Second / 60)
	}
	var lefts []float64
	for q.All(); q.Next(); {
		for _, b := range base.Slice(q.Cursor()) {
			lefts = append(lefts, b.Pos.AABB.TopLeft.X)
		}
	}
	if len(lefts) != 1 || lefts[0] != 0 {
		t.Errorf("boxes left at x %v, want the wall at 0 alone", lefts)
	}
}
