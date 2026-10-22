package bench_test

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/bullet"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
)

// the shots' scene: a 1024-px square, a shot a second in the air from the left edge rightwards
const (
	shotScene = 1024
	shotSpeed = 400.0
	shotRange = 400.0
)

type shotWorld struct {
	ecs     *goke.ECS
	w       *world.Plugin
	ammo    bullet.Ammo
	climb   float64
	rng     *rand.Rand
	query   *goke.Query
	flights goke.Comp[bullet.Flight]
}

func benchBullets(b *testing.B, thrown bool, perTick int) *shotWorld {
	b.Helper()
	ctx := newHeadless()
	s := &shotWorld{rng: rand.New(rand.NewPCG(0x5eed, 0xc0ffee))}
	s.w = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: shotScene, Height: shotScene},
		Entities: world.EntitiesCfg{MaxCount: 2 * perTick * 60, MinSize: 1, MaxSize: 8},
	})
	m := motion.NewPlugin(s.w)
	if err := ctx.Use(m); err != nil {
		b.Fatal(err)
	}
	c := collision.NewPlugin(s.w, m)
	if err := ctx.Use(c); err != nil {
		b.Fatal(err)
	}
	sel := selection.NewPlugin(s.w)
	if err := ctx.Use(sel); err != nil {
		b.Fatal(err)
	}
	bl := bullet.NewPlugin(s.w, m, sel)
	if err := ctx.Use(bl); err != nil {
		b.Fatal(err)
	}
	body := bullet.Body{Size: 4, Speed: shotSpeed, Range: shotRange}
	if thrown {
		body.Gravity = 300
		s.climb = body.Gravity * (shotRange / shotSpeed) / 2 // as the shoot system throws at the ground
	}
	bl.Shots().Define("ball", body)
	s.ammo = bl.Shots().Named("ball")
	ctx.pending = append(ctx.pending, func() []goke.System {
		return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
			s.query = si.NewQueryBuilder(&s.flights).Build()
		}}}
	})
	s.ecs = ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		bl.RunPlan(rc, d)
		s.w.RunPlan(rc, d)
		m.RunPlan(rc, d)
		c.RunPlan(rc, d)
		rc.Sync()
	})
	for range 120 {
		s.fire(perTick)
		s.ecs.Tick(step)
	}
	return s
}

// fire spawns n shots from the left edge, flying right.
func (s *shotWorld) fire(n int) {
	entries := make([]kind.Entry, 0, n)
	for range n {
		from := geom.NewVec(8, 8+s.rng.Float64()*(shotScene-16))
		entries = append(entries, s.ammo.Entry(bullet.Shot{
			From: from, Dir: geom.NewVec(1, 0), Range: shotRange, Climb: s.climb,
		}))
	}
	s.w.Write().Spawn(entries...)
}

// inFlight counts the shots in the air.
func (s *shotWorld) inFlight() int {
	n := 0
	for s.query.All(); s.query.Next(); {
		for _, f := range s.flights.Slice(s.query.Cursor()) {
			if !f.Landed {
				n++
			}
		}
	}
	return n
}

// Benchmark_Bullet_Flight is one tick of the bullet, world, motion and collision plugins with
// shots fired every tick, each a second in the air: straight, or thrown and coming down on
// the ground at their range, where they are despawned.
func Benchmark_Bullet_Flight(b *testing.B) {
	for _, sc := range []struct {
		name   string
		thrown bool
	}{{"straight", false}, {"thrown", true}} {
		for _, perTick := range []int{1, 5, 20} {
			b.Run(fmt.Sprintf("%s,shots/tick=%d", sc.name, perTick), func(b *testing.B) {
				s := benchBullets(b, sc.thrown, perTick)
				b.ReportAllocs()
				for b.Loop() {
					s.fire(perTick)
					s.ecs.Tick(step)
				}
				b.ReportMetric(float64(s.inFlight()), "inflight")
			})
		}
	}
}
