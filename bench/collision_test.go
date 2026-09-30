package bench_test

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/trigger"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
)

// The collision scenes are the collision demo's: a 1024x1024 torus filled to a share of its
// area with square boxes of one side, every box bouncing off every other.
const (
	sceneWidth  = 1024
	sceneHeight = 1024
	sceneTPS    = 120
)

// countFor is how many boxes of side rect cover percent of the scene.
func countFor(rect uint32, percent float64) int {
	return int(math.Floor(percent / 100.0 * float64(sceneWidth*sceneHeight) / float64(rect*rect)))
}

// body is the row every bouncing box spawns from.
type body struct {
	pos world.Position
	vel world.Velocity
}

// randomVelocity draws each component from [-200, 200], keeping |dx| at least 10.
func randomVelocity(rng *rand.Rand) world.Velocity {
	dx := rng.Int32N(401) - 200
	dy := rng.Int32N(401) - 200
	if dx >= 0 && dx < 50 {
		dx = 10
	} else if dx < 0 && dx > -50 {
		dx = -10
	}
	var vel world.Velocity
	vel.SetDelta(geom.NewVec(float64(dx), float64(dy)))
	return vel
}

// benchCollision installs a world and a collision plugin counting every contact, spawns the
// scene on a grid with seeded random velocities, and runs 120 ticks so the boxes have spread.
func benchCollision(b *testing.B, rect uint32, percent float64) (*goke.ECS, int, *trigger.ContactStats) {
	b.Helper()
	count := countFor(rect, percent)
	rng := rand.New(rand.NewPCG(0x5eed, 0xc0ffee))

	ctx := newHeadless()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: sceneWidth, Height: sceneHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: count, MinSize: rect, MaxSize: rect},
	})
	c := collision.NewPlugin(w)
	stats := &trigger.ContactStats{}
	if err := c.Hook(act.Trigger[collision.Meeting]("count contacts").Do(trigger.CountContacts(stats))); err != nil {
		b.Fatal(err)
	}
	if err := ctx.Use(c); err != nil {
		b.Fatal(err)
	}
	boxes := kind.Define[body](w.Kinds(), "box", kind.Spec{
		comp.Load(func(r body) world.Position { return r.pos }),
		comp.Load(func(r body) world.Velocity { return r.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
	})
	placement := world.NewGridPlacement(sceneWidth, sceneHeight, rect)
	entries := make([]kind.Entry, count)
	for i := range entries {
		entries[i] = boxes.Entry(body{pos: placement.Place(i, count), vel: randomVelocity(rng)})
	}
	w.Seed(entries...)

	ecs := ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		c.RunPlan(rc, d)
		rc.Sync()
	})
	for range 120 {
		ecs.Tick(time.Second / sceneTPS)
	}
	return ecs, count, stats
}

// Benchmark_Collision_Tick is one tick of world plus collision at the demo's scales: movement,
// the space rebuilt, every overlapping pair found, tested, bounced and pushed apart.
func Benchmark_Collision_Tick(b *testing.B) {
	for _, sc := range []struct {
		name    string
		rect    uint32
		percent float64
	}{
		{"rect=20,fill=20%", 20, 20},
		{"rect=10,fill=20%", 10, 20},
		{"rect=20,fill=40%", 20, 40},
		{"rect=10,fill=40%", 10, 40},
		{"rect=8,fill=20%", 8, 20},
		{"rect=5,fill=20%", 5, 20},
	} {
		b.Run(sc.name, func(b *testing.B) {
			ecs, count, stats := benchCollision(b, sc.rect, sc.percent)
			before := stats.Counter
			ticks := 0
			b.ReportAllocs()
			for b.Loop() {
				ecs.Tick(time.Second / sceneTPS)
				ticks++
			}
			b.ReportMetric(float64(count), "entities")
			b.ReportMetric(float64(stats.Counter-before)/float64(ticks), "contacts/tick")
		})
	}
}
