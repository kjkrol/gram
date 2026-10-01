package world_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/rule"
	"github.com/kjkrol/gram/plugins/world/rule/effect"
	"github.com/kjkrol/uid"
)

// roles is the tag family of the rule tests.
type roles struct{}

// spot is where a rule test's unit starts, which way it moves, and whether it is the bullet.
type spot struct {
	x, vx  float64
	bullet bool
}

// triggerStage is a world with collision and units at spots; hook hooks its rules.
type triggerStage struct {
	spots          []spot
	hook           func(g *triggerStage) error
	world          *world.Plugin
	coll           *collision.Plugin
	bullet, target tag.Tag[roles]
	unit           kind.Of[spot]
	base           goke.Comp[world.Base]
	active         goke.OptComp[effect.Active]
	marks          goke.OptComp[tag.Tags[roles]]
	query          *goke.Query
	stack          game.Scenes
}

func (g *triggerStage) Name() string { return "stage" }

func (g *triggerStage) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	g.bullet = g.world.Kinds().DefineTag[roles]("bullet")
	g.target = g.world.Kinds().DefineTag[roles]("target")
	ctx.Setup(triggerProbe{g})
	g.coll = collision.NewPlugin(g.world)
	if err := g.hook(g); err != nil {
		return err
	}
	g.unit = kind.Define[spot](g.world.Kinds(), "unit", kind.Spec{
		comp.Load(func(s spot) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(s.x, 100), 10, 10)}
		}),
		comp.Load(func(s spot) world.Velocity { return world.Velocity{Dir: geom.NewVec(1, 0), Value: s.vx} }),
		comp.Const(collision.Collider{}),
		comp.Load(func(s spot) tag.Tags[roles] {
			if s.bullet {
				return tag.Tags[roles](0).With(g.bullet)
			}
			return tag.Tags[roles](0).With(g.target)
		}),
	})
	return ctx.Use(g.coll)
}

type triggerProbe struct{ g *triggerStage }

func (p triggerProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.g.query = si.NewQueryBuilder(&p.g.base).Optional(&p.g.active, &p.g.marks).Build()
	}}}
}

func (g *triggerStage) Restore(game.Persistence) (bool, error) { return false, nil }
func (g *triggerStage) Spawn() error {
	for _, s := range g.spots {
		g.world.Seed(g.unit.Entry(s))
	}
	return nil
}
func (g *triggerStage) Update(ctx goke.RunCtx, d time.Duration) {
	g.world.RunPlan(ctx, d)
	g.coll.RunPlan(ctx, d)
}
func (g *triggerStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

// each calls fn with every unit: where it is, whether it is the bullet, and how many of e it holds.
func (g *triggerStage) each(e effect.Effect, fn func(id uid.UID64, x float64, bullet bool, under int)) {
	for g.query.All(); g.query.Next(); {
		cur := g.query.Cursor()
		actives, marks := g.active.Slice(cur), g.marks.Slice(cur)
		for i, id := range cur.IDs {
			n := 0
			if actives != nil {
				for _, s := range actives[i].Slots {
					if s.State != effect.Empty && s.Kind == e.ID() {
						n++
					}
				}
			}
			fn(id, g.base.Slice(cur)[i].Pos.TopLeft.X, marks != nil && marks[i].Has(g.bullet), n)
		}
	}
}

func runTriggers(t *testing.T, g *triggerStage, d time.Duration) {
	t.Helper()
	e := engine.NewEngine(oneStageGame{stage: g, props: game.Props{}})
	if err := e.Init(); err != nil {
		t.Fatal(err)
	}
	step(t, e, d)
}

// Self narrows a rule on a moment that is not a pair to the entities carrying the tag: only the
// bullet is held still.
func TestRule_SelfNarrowsToTheTaggedEntities(t *testing.T) {
	g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 500, vx: 60}}}
	g.hook = func(g *triggerStage) error {
		return g.world.Hook(rule.On("hold the bullet", rule.Self(g.bullet), func(m *rule.Moment[world.Moving]) rule.Step {
			return m.Call(func(_ plugin.Tick, m world.Moving) { m.Base.Vel.Value = 0 })
		}))
	}
	runTriggers(t, g, 300*time.Millisecond)
	g.each(effect.Effect{}, func(_ uid.UID64, x float64, bullet bool, _ int) {
		if moved := x != 100 && x != 500; moved == bullet {
			t.Errorf("the bullet %v moved %v (at %v); want the bullet held, the other moving", bullet, moved, x)
		}
	})
}

// A pair's rule turns ToOther on whom the entity met: the bullet striking the target marks
// the target, not itself.
func TestRule_ForOtherActsOnWhomTheEntityMet(t *testing.T) {
	var marked effect.Effect
	g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 125}}}
	g.hook = func(g *triggerStage) error {
		marked = g.world.Effects().Define("marked", effect.Spec{effect.Lasts(time.Hour)})
		return g.coll.Hook(rule.On("mark the target", rule.Between(g.bullet, g.target), func(m *rule.Moment[collision.Meeting]) rule.Step {
			return m.ForOther(m.Apply(marked))
		}))
	}
	runTriggers(t, g, 700*time.Millisecond)
	g.each(marked, func(_ uid.UID64, _ float64, bullet bool, under int) {
		if (under > 0) == bullet {
			t.Errorf("the bullet %v is marked %v; want the target marked alone", bullet, under > 0)
		}
	})
}

// An effect is a rule's memory: with Unless on a mark that lasts 400 ms, what it guards runs
// about once in 400 ms.
func TestRule_AnEffectIsItsMemory(t *testing.T) {
	var mark effect.Effect
	count := 0
	g := &triggerStage{spots: []spot{{x: 100, vx: 1}}}
	g.hook = func(g *triggerStage) error {
		mark = g.world.Effects().Define("mark", effect.Spec{effect.Lasts(400 * time.Millisecond)})
		return g.world.Hook(rule.On("once a while", rule.All, func(m *rule.Moment[world.Moving]) rule.Step {
			return m.Unless(mark, m.Steps(m.Apply(mark), m.Call(func(plugin.Tick, world.Moving) { count++ })))
		}))
	}
	runTriggers(t, g, time.Second)
	if count < 2 || count > 4 {
		t.Errorf("in a second the guarded body ran %d times, want about once in 400 ms: 2 to 4", count)
	}
}

// A rule issues commands as a tree does, through its host's Tick: the bullet on the move gives
// itself a Despawn in the world's pass and is gone.
func TestRule_OrdersCommands(t *testing.T) {
	g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 500, vx: 60}}}
	g.hook = func(g *triggerStage) error {
		return g.world.Hook(rule.On("the bullet leaves", rule.Self(g.bullet), func(m *rule.Moment[world.Moving]) rule.Step {
			return m.Order(world.Despawn{})
		}))
	}
	runTriggers(t, g, 200*time.Millisecond)
	n := 0
	g.each(effect.Effect{}, func(_ uid.UID64, _ float64, bullet bool, _ int) {
		n++
		if bullet {
			t.Error("the bullet is still in the world, want it despawned by its own command")
		}
	})
	if n != 1 {
		t.Errorf("%d units in the world, want the target alone", n)
	}
}
