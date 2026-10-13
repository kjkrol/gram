package bench_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/fact"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

type glowing struct{}

type pulse struct{ N int32 }

func (pulse) Fact() {}

var _ fact.Fact = pulse{}

// behaviour is a world of n still units, a sixteenth of them told a pulse every tick and
// carrying it eight, for the benchmarks of the rules, the plans and the effects.
type behaviour struct {
	ctx    *headless
	w      *world.Plugin
	ecs    *goke.ECS
	glow   effect.Effect
	spark  effect.Effect
	ids    []uid.UID64
	driver goke.Runnable
	tick   int
	pulse  goke.CompID
	first  func(cb *goke.CmdBuf)
}

func newBehaviour(b *testing.B, n int, define func(bh *behaviour) kind.Spec) *behaviour {
	b.Helper()
	bh := &behaviour{ctx: newHeadless()}
	bh.w = bh.ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 4000, Height: 4000, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: n, MinSize: 1, MaxSize: 100},
	})
	fx := bh.w.Effects()
	lit := bh.w.Kinds().DefineTag[glowing]("lit")
	fx.Define("glow", effect.Spec{
		effect.Lasts(3 * step),
		effect.Grant(lit),
		effect.Alter(func(s *steering.Steering) { s.MaxSpeed *= 0.5 }),
	})
	fx.Define("spark", effect.Spec{effect.Lasts(step)})
	bh.glow = fx.Named("glow")
	bh.spark = fx.Named("spark")
	spec := append(kind.Spec{
		comp.Load(func(m mover) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(m.x, m.y), 20, 20)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(steering.Steering{MaxSpeed: 60}),
		comp.Const(steering.Course{}),
		comp.Marks[effect.States](),
		comp.Tagged[glowing](),
	}, define(bh)...)
	kind.Define[mover](bh.w.Kinds(), "mover", spec)
	movers := kind.Named[mover](bh.w.Kinds(), "mover")
	side := 1
	for side*side < n {
		side++
	}
	entries := make([]kind.Entry, 0, n)
	for i := range n {
		entries = append(entries, movers.Entry(mover{float64(10 + (i%side)*30), float64(10 + (i/side)*30)}))
	}
	bh.w.Seed(entries...)
	if err := bh.ctx.Deliver(bh.w.Kinds().Played()...); err != nil {
		b.Fatal(err)
	}
	var base goke.Comp[world.Base]
	var all *goke.Query
	bh.ctx.pending = append(bh.ctx.pending, func() []goke.System {
		return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
			bh.driver = bh.ctx.ecs.RegSys(goke.SystemFn{
				OnInit: func(si *goke.SysInit) {
					bh.pulse = si.RegComp[pulse]()
					all = si.NewQueryBuilder(&base).Build()
				},
				OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
					if bh.ids == nil {
						for all.All(); all.Next(); {
							bh.ids = append(bh.ids, all.Cursor().IDs...)
						}
						if bh.first != nil {
							bh.first(cb)
						}
					}
					bh.tick++
					for i, id := range bh.ids { // a pulse carried eight ticks, a sixteenth told a tick
						switch {
						case i%16 == bh.tick%16:
							cb.AddOne(id, bh.pulse, pulse{N: int32(bh.tick)})
						case (i+8)%16 == bh.tick%16:
							cb.RemoveCompOne(id, bh.pulse)
						}
					}
				},
			})
		}}}
	})
	bh.ecs = bh.ctx.start(b, func(rc goke.RunCtx, d time.Duration) {
		rc.Run(bh.driver, d)
		rc.Sync()
		bh.w.RunPlan(rc, d)
	})
	for range 4 {
		bh.ecs.Tick(step)
	}
	return bh
}

func (bh *behaviour) loop(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		bh.ecs.Tick(step)
	}
}

// Benchmark_Plans_Tick is one tick of n units each on a plan of two loops: one idling until a
// pulse comes afresh, one begun by the pulse when its word holds, putting an effect on and
// waiting a step.
func Benchmark_Plans_Tick(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		b.Run(entities(n), func(b *testing.B) {
			strong := cond.NewWord("strong", func(p pulse) bool { return p.N > 0 })
			bh := newBehaviour(b, n, func(bh *behaviour) kind.Spec {
				return kind.Spec{plan.New("pulsed", plan.Steps(
					plan.Loop[any]{
						Name:  "await",
						When:  cond.Self.If(cond.Always),
						Then:  plan.Self.Idle(),
						Until: cond.Self.If(pulse{}),
					},
					plan.Loop[pulse]{
						Name: "react",
						When: cond.Self.If(strong).And(cond.Self.If(cond.Not(bh.spark))),
						Then: plan.Steps(plan.Self.Put(bh.spark), plan.Self.Wait(step)),
					},
				))}
			})
			bh.loop(b)
		})
	}
}

// Benchmark_Rules_Moving is one tick of n units playing a role of two rules of world.Moving:
// one holding an effect while a word of the moment holds, one putting another on by chance
// while under it.
func Benchmark_Rules_Moving(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		b.Run(entities(n), func(b *testing.B) {
			still := cond.NewWord("still", func(m world.Moving) bool { return m.Base.Vel.Value == 0 })
			bh := newBehaviour(b, n, func(bh *behaviour) kind.Spec {
				bh.w.Roles().Define("glower",
					act.Branch[world.Moving]{
						Name: "glow",
						When: cond.Self.If(still),
						Then: act.Self.Hold(bh.glow),
					},
					act.Branch[world.Moving]{
						Name: "spark",
						When: cond.Self.If(bh.glow).And(cond.Self.If(cond.Chance(0.1))),
						Then: act.Self.Put(bh.spark),
					})
				return kind.Spec{ceremony.Plays(bh.w.Roles().Named("glower"))}
			})
			bh.loop(b)
		})
	}
}

// Benchmark_Effects_Tick is one tick of n units under an effect lasting three steps with a
// Grant and an Alter, cast again by a plan as it ends, so a third of them begin or end a step.
func Benchmark_Effects_Tick(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		b.Run(entities(n), func(b *testing.B) {
			bh := newBehaviour(b, n, func(bh *behaviour) kind.Spec {
				return kind.Spec{plan.New("reglow", plan.Loop[any]{
					Name: "reglow",
					When: cond.Self.If(cond.Not(bh.glow)),
					Then: plan.Self.Put(bh.glow),
				})}
			})
			bh.first = func(cb *goke.CmdBuf) {
				for i, id := range bh.ids {
					bh.w.Effects().CastFor(id, bh.glow, time.Duration(1+i%3)*step)
				}
			}
			bh.loop(b)
		})
	}
}
