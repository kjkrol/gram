package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/world"
)

type runner struct {
	x, side float64
	heading float64 // +1 or -1 along x
}

func TestWorldAndCollisions_MixedSizes_NeverTunnel(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 2, MaxSize: 100},
	})
	c := collision.NewPlugin(w)
	kind.Define[runner](w.Kinds(), "runner", kind.Spec{
		comp.Load(func(r runner) world.Position { return posAt(r.x, 500-r.side/2, r.side, r.side) }),
		comp.Load(func(r runner) world.Velocity {
			return world.Velocity{Dir: geom.NewVec(r.heading, 0), Value: 100000}
		}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{}),
	})
	runners := kind.Named[runner](w.Kinds(), "runner")
	small, big := runner{x: 1000, side: 2, heading: 1}, runner{x: 1600, side: 100, heading: -1}
	w.Seed(runners.Entry(small), runners.Entry(big))
	if err := w.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	var base goke.Comp[world.Base]
	var q *goke.Query
	ecs := collisiontest.Start(t, w, c, goke.SystemFn{OnInit: func(si *goke.SysInit) { q = si.NewQueryBuilder(&base).Build() }})

	where := func() (smallX, bigX float64) {
		for q.All(); q.Next(); {
			for _, b := range base.Slice(q.Cursor()) {
				if b.Pos.Size.X == small.side {
					smallX = b.Pos.TopLeft.X
				} else {
					bigX = b.Pos.TopLeft.X
				}
			}
		}
		return
	}

	ecs.Tick(time.Second / 60)
	smallX, bigX := where()
	if got := smallX - small.x; got != 1 {
		t.Errorf("the 2-unit entity moved %v in its first tick, want 1", got)
	}
	if got := big.x - bigX; got != 50 {
		t.Errorf("the 100-unit entity moved %v in its first tick, want 50 — the small one must not cap it", got)
	}

	for tick := range 120 {
		ecs.Tick(time.Second / 60)
		smallX, bigX = where()
		if smallX+small.side > bigX+big.side/2 {
			t.Fatalf("tick %d: the small entity is at %v, past the middle of the big one at %v — it tunnelled", tick, smallX, bigX)
		}
	}
}
