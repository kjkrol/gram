package bench_test

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
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
func benchCollision(b *testing.B, rect uint32, percent float64) (*goke.ECS, int, *collision.ContactStats) {
	b.Helper()
	count := countFor(rect, percent)
	rng := rand.New(rand.NewPCG(0x5eed, 0xc0ffee))

	ctx := newHeadless()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: sceneWidth, Height: sceneHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: count, MinSize: rect, MaxSize: rect},
	})
	stats := &collision.ContactStats{}
	c := collision.NewPlugin(w).WithStats(stats)
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

// flight is what a swept shot of the bench does every tick: fly Step along Dir, turning back at
// the world's edge.
type flight struct {
	Dir  geom.Vec
	Step float64
}

// flyer moves every swept shot before the world's pass, as a bullet plugin would: it writes the
// Sweep's From and flies the shot its step.
type flyer struct {
	q     *goke.Query
	base  goke.Comp[world.Base]
	sweep goke.Comp[collision.Sweep]
	fl    goke.Comp[flight]
}

func (f *flyer) Init(si *goke.SysInit) {
	f.q = si.NewQueryBuilder(&f.base, &f.sweep, &f.fl).Build()
}

func (f *flyer) Update(*goke.CmdBuf, time.Duration) {
	for f.q.All(); f.q.Next(); {
		cur := f.q.Cursor()
		bases, sweeps, flights := f.base.Slice(cur), f.sweep.Slice(cur), f.fl.Slice(cur)
		for i := range cur.IDs {
			pos, fl := &bases[i].Pos, &flights[i]
			sweeps[i].From = pos.Center()
			to := geom.NewVec(pos.TopLeft.X+fl.Dir.X*fl.Step, pos.TopLeft.Y+fl.Dir.Y*fl.Step)
			if to.X < 0 || to.X+pos.Size.X > sceneWidth {
				fl.Dir.X = -fl.Dir.X
				to.X = pos.TopLeft.X + fl.Dir.X*fl.Step
			}
			pos.AABB = plane.NewAABB(to, pos.Size.X, pos.Size.Y)
		}
	}
}

// shot is the row a swept shot spawns from.
type shot struct {
	pos world.Position
	dir geom.Vec
}

// benchSwept is benchCollision's scene of 10-px boxes at 20%, in a closed world, with shots of 4
// flying 60 a tick across it, swept: every tick every shot is paired with all on its path and
// refined to its segment.
func benchSwept(b *testing.B, shots int) (*goke.ECS, *collision.ContactStats) {
	b.Helper()
	const rect, percent = 10, 20.0
	count := countFor(rect, percent)
	rng := rand.New(rand.NewPCG(0x5eed, 0xc0ffee))

	ctx := newHeadless()
	w := ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: sceneWidth, Height: sceneHeight},
		Entities: world.EntitiesCfg{MaxCount: count + shots, MinSize: 4, MaxSize: rect},
	})
	stats := &collision.ContactStats{}
	c := collision.NewPlugin(w).WithStats(stats)
	if err := ctx.Use(c); err != nil {
		b.Fatal(err)
	}
	boxes := kind.Define[body](w.Kinds(), "box", kind.Spec{
		comp.Load(func(r body) world.Position { return r.pos }),
		comp.Load(func(r body) world.Velocity { return r.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
	})
	bullets := kind.Define[shot](w.Kinds(), "shot", kind.Spec{
		comp.Load(func(r shot) world.Position { return r.pos }),
		comp.Const(world.Velocity{}),
		comp.Const(collision.Collider{}),
		comp.Load(func(r shot) collision.Sweep { return collision.Sweep{From: r.pos.Center()} }),
		comp.Load(func(r shot) flight { return flight{Dir: r.dir, Step: 60} }),
	})
	placement := world.NewGridPlacement(sceneWidth, sceneHeight, rect)
	entries := make([]kind.Entry, 0, count+shots)
	for i := range count {
		entries = append(entries, boxes.Entry(body{pos: placement.Place(i, count), vel: randomVelocity(rng)}))
	}
	for i := range shots {
		y := float64(i+1) * sceneHeight / float64(shots+1)
		var pos world.Position
		pos.AABB = plane.NewAABB(geom.NewVec(2, y), 4, 4)
		entries = append(entries, bullets.Entry(shot{pos: pos, dir: geom.NewVec(1, 0)}))
	}
	w.Seed(entries...)

	var fly goke.Runnable
	ecs := ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		rc.Run(fly, d)
		rc.Sync()
		w.RunPlan(rc, d)
		c.RunPlan(rc, d)
		rc.Sync()
	})
	fly = ecs.RegSys(&flyer{})
	for range 120 {
		ecs.Tick(time.Second / sceneTPS)
	}
	return ecs, stats
}

// Benchmark_Collision_Swept is Benchmark_Collision_Tick's 10-px scene with swept shots crossing
// it: the space rebuilt with their stretches and without, every pair on a path refined.
func Benchmark_Collision_Swept(b *testing.B) {
	for _, shots := range []int{0, 1, 16, 64} {
		b.Run(fmt.Sprintf("shots=%d", shots), func(b *testing.B) {
			ecs, stats := benchSwept(b, shots)
			before := stats.Counter
			ticks := 0
			b.ReportAllocs()
			for b.Loop() {
				ecs.Tick(time.Second / sceneTPS)
				ticks++
			}
			b.ReportMetric(float64(stats.Counter-before)/float64(ticks), "contacts/tick")
		})
	}
}
