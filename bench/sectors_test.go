package bench_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// the expanse demo's scene: a torus 16384 a side, its boxes 5 a side, a tick a step
const (
	expanseSide  = 1 << 14
	expanseBoxes = 100_000
	expanseTPS   = 60
)

type anchor struct{ x, y float32 }

var _ sector.Anchors = anchor{}

func (a anchor) Anchors(at func(x, y float32)) { at(a.x, a.y) }

func benchExpanse(b *testing.B, banded bool) *goke.ECS {
	b.Helper()
	rng := rand.New(rand.NewPCG(0x5eed, 0xc0ffee))
	ctx := newHeadless()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: expanseSide, Height: expanseSide, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: expanseBoxes, MinSize: 5, MaxSize: 5},
		Sectors: world.SectorsCfg{Buckets: 8, Bands: []sector.Band{
			{Reach: 750},
			{Reach: 2000, Pace: 4},
		}},
	})
	w.Write().Anchor(anchor{x: expanseSide / 2, y: expanseSide / 2})
	w.Write().SetBanded(banded)
	m := motion.NewPlugin(w)
	if err := ctx.Use(m); err != nil {
		b.Fatal(err)
	}
	c := collision.NewPlugin(w, m)
	if err := ctx.Use(c); err != nil {
		b.Fatal(err)
	}
	kind.Define[body](w.Kinds(), "box", kind.Spec{
		comp.Load(func(r body) world.Position { return r.pos }),
		comp.Load(func(r body) world.Velocity { return r.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
	})
	boxes := kind.Named[body](w.Kinds(), "box")
	placement := world.NewGridPlacement(expanseSide, expanseSide, 5)
	entries := make([]kind.Entry, expanseBoxes)
	for i := range entries {
		entries[i] = boxes.Entry(body{pos: placement.Place(i, expanseBoxes),
			vel: randomVelocity(rng)})
	}
	w.Write().Seed(entries...)
	ecs := ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		c.RunPlan(rc, d)
		rc.Sync()
	})
	for range 8 {
		ecs.Tick(time.Second / expanseTPS)
	}
	return ecs
}

// Benchmark_Sectors_Expanse is one tick of world, motion and collision at the expanse demo's
// scale, its bands round the middle and switched off.
func Benchmark_Sectors_Expanse(b *testing.B) {
	for _, sc := range []struct {
		name   string
		banded bool
	}{
		{"banded", true},
		{"unbanded", false},
	} {
		b.Run(sc.name, func(b *testing.B) {
			ecs := benchExpanse(b, sc.banded)
			b.ReportAllocs()
			for b.Loop() {
				ecs.Tick(time.Second / expanseTPS)
			}
			b.ReportMetric(expanseBoxes, "entities")
		})
	}
}
