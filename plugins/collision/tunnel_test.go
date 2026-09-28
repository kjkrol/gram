package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
)

// installCtx is the least of a plugin.Installer that world and collision need.
type installCtx struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}

func (c *installCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installCtx) RegSys(f func() goke.System) goke.Runnable { return c.ecs.RegSys(f()) }
func (c *installCtx) ECS() *goke.ECS                            { return c.ecs }

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
	ctx := &installCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatalf("world Install: %v", err)
	}
	if err := c.Install(ctx); err != nil {
		t.Fatalf("collision Install: %v", err)
	}

	runners := kind.Define[runner](w.Kinds(), "runner", kind.Spec{
		comp.Load(func(r runner) world.Position { return posAt(r.x, 500-r.side/2, r.side, r.side) }),
		comp.Load(func(r runner) world.Velocity {
			return world.Velocity{Dir: geom.NewVec(r.heading, 0), Value: 100000}
		}),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{}),
	})
	small, big := runner{x: 1000, side: 2, heading: 1}, runner{x: 1600, side: 100, heading: -1}
	w.Seed(runners.Entry(small), runners.Entry(big))
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
		w.Clock().Replay(rc, d)
	})

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

	ctx.ecs.Tick(time.Second / 60)
	smallX, bigX := where()
	if got := smallX - small.x; got != 1 {
		t.Errorf("the 2-unit entity moved %v in its first tick, want 1", got)
	}
	if got := big.x - bigX; got != 50 {
		t.Errorf("the 100-unit entity moved %v in its first tick, want 50 — the small one must not cap it", got)
	}

	for tick := range 120 {
		ctx.ecs.Tick(time.Second / 60)
		smallX, bigX = where()
		if smallX+small.side > bigX+big.side/2 {
			t.Fatalf("tick %d: the small entity is at %v, past the middle of the big one at %v — it tunnelled", tick, smallX, bigX)
		}
	}
}
