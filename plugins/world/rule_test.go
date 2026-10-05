package world_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// roles is the tag family of the rule tests.
type roles struct{}

// spot is where a rule test's unit starts, which way it moves, and whether it is the bullet.
type spot struct {
	x, vx  float64
	bullet bool
}

// triggerStage is a world with collision and units at spots; hook gives the rules its units obey
// (obey), all playing one role.
type triggerStage struct {
	spots          []spot
	hook           func(g *triggerStage) error
	rules          []rule.Rule
	world          *world.Plugin
	coll           *collision.Plugin
	bullet, target tag.Tag[roles]
	unit           kind.Of[spot]
	base           goke.Comp[world.Base]
	states         goke.OptComp[tag.Tags[effect.States]]
	tallies        goke.OptComp[tally]
	marks          goke.OptComp[tag.Tags[roles]]
	query          *goke.Query
	stack          game.Scenes
}

func (g *triggerStage) Name() string { return "stage" }

// obey has every unit obey rules.
func (g *triggerStage) obey(rules ...rule.Rule) error {
	g.rules = append(g.rules, rules...)
	return nil
}

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
		comp.Const(tally{}),
		rule.Plays(rule.Role("trigger unit").Obeys(g.rules...)),
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
		p.g.query = si.NewQueryBuilder(&p.g.base).Optional(&p.g.states, &p.g.tallies, &p.g.marks).Build()
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

// each calls fn with every unit: where it is, whether it is the bullet, and whether it is under
// e, its marker on.
func (g *triggerStage) each(e effect.Effect, fn func(id uid.UID64, x float64, bullet bool, under bool)) {
	for g.query.All(); g.query.Next(); {
		cur := g.query.Cursor()
		states, marks := g.states.Slice(cur), g.marks.Slice(cur)
		for i, id := range cur.IDs {
			under := e != (effect.Effect{}) && states != nil && states[i].Has(e.Mark())
			fn(id, g.base.Slice(cur)[i].Pos.TopLeft.X, marks != nil && marks[i].Has(g.bullet), under)
		}
	}
}

// tallied is how many of the counting effects the units run, together.
func (g *triggerStage) tallied() int {
	n := 0
	for g.query.All(); g.query.Next(); {
		for _, t := range g.tallies.Slice(g.query.Cursor()) {
			n += t.N
		}
	}
	return n
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
// bullet is marked.
func TestRule_SelfNarrowsToTheTaggedEntities(t *testing.T) {
	var marked effect.Effect
	g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 500, vx: 60}}}
	g.hook = func(g *triggerStage) error {
		g.world.Effects().Define("marked", effect.Spec{effect.Lasts(time.Hour)})
		marked = g.world.Effects().Named("marked")
		return g.obey(rule.Then[world.Moving]("mark the bullet", rule.Self(g.bullet), rule.Apply(marked)))
	}
	runTriggers(t, g, 300*time.Millisecond)
	g.each(marked, func(_ uid.UID64, _ float64, bullet bool, under bool) {
		if under != bullet {
			t.Errorf("the bullet %v is marked %v; want the bullet marked alone", bullet, under)
		}
	})
}

// Having narrows a rule to the entities carrying a component: only the hit one is marked.
func TestRule_HavingNarrowsToTheEntitiesWithTheComponent(t *testing.T) {
	var marked effect.Effect
	g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 500, vx: 60}}}
	g.hook = func(g *triggerStage) error {
		g.world.Effects().Define("marked", effect.Spec{effect.Lasts(time.Hour)})
		marked = g.world.Effects().Named("marked")
		return g.obey(rule.Then[world.Moving]("mark the colliders", rule.Having[collision.Collider](), rule.Apply(marked)))
	}
	runTriggers(t, g, 300*time.Millisecond)
	n := 0
	g.each(marked, func(_ uid.UID64, _ float64, _ bool, under bool) {
		if under {
			n++
		}
	})
	if n != 2 {
		t.Errorf("%d marked, want both: both carry a Collider", n)
	}
}

// A pair's rule turns ToOther on whom the entity met: the bullet striking the target marks
// the target, not itself.
func TestRule_ForOtherActsOnWhomTheEntityMet(t *testing.T) {
	var marked effect.Effect
	g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 125}}}
	g.hook = func(g *triggerStage) error {
		g.world.Effects().Define("marked", effect.Spec{effect.Lasts(time.Hour)})
		marked = g.world.Effects().Named("marked")
		return g.obey(rule.Then[collision.Meeting]("mark the target", rule.Between(g.bullet, g.target), rule.ForOther(rule.Apply(marked))))
	}
	runTriggers(t, g, 700*time.Millisecond)
	g.each(marked, func(_ uid.UID64, _ float64, bullet bool, under bool) {
		if under == bullet {
			t.Errorf("the bullet %v is marked %v; want the target marked alone", bullet, under)
		}
	})
}

// An effect is a rule's memory: with Unless on a mark that lasts 400 ms, what it guards runs
// about once in 400 ms.
func TestRule_AnEffectIsItsMemory(t *testing.T) {
	var mark, tallying effect.Effect
	g := &triggerStage{spots: []spot{{x: 100, vx: 1}}}
	g.hook = func(g *triggerStage) error {
		g.world.Effects().Define("mark", effect.Spec{effect.Lasts(400 * time.Millisecond)})
		mark = g.world.Effects().Named("mark")
		g.world.Effects().Define("tally", effect.Spec{effect.Lasts(time.Hour), effect.Stacking(), counting})
		tallying = g.world.Effects().Named("tally")
		return g.obey(rule.Then[world.Moving]("once a while", rule.All, rule.Unless(mark, rule.Steps(rule.Apply(mark), rule.Apply(tallying)))))
	}
	runTriggers(t, g, time.Second)
	if count := g.tallied(); count < 2 || count > 4 {
		t.Errorf("in a second the guarded body ran %d times, want about once in 400 ms: 2 to 4", count)
	}
}

// A rule issues commands as a tree does, through its host's Tick: the bullet on the move gives
// itself a Despawn in the world's pass and is gone.
func TestRule_OrdersCommands(t *testing.T) {
	g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 500, vx: 60}}}
	g.hook = func(g *triggerStage) error {
		return g.obey(rule.Then[world.Moving]("the bullet leaves", rule.Self(g.bullet), rule.Order(world.Despawn{})))
	}
	runTriggers(t, g, 200*time.Millisecond)
	n := 0
	g.each(effect.Effect{}, func(_ uid.UID64, _ float64, bullet bool, _ bool) {
		n++
		if bullet {
			t.Error("the bullet is still in the world, want it despawned by its own command")
		}
	})
	if n != 1 {
		t.Errorf("%d units in the world, want the target alone", n)
	}
}

// An effect's marker is a filter in any plugin: the bullet set burning by a world's rule sets the
// target burning by a collision's, filtered by burning's marker alone; with nobody burning, nobody
// catches fire.
func TestRule_AnEffectsMarkerFiltersInAnotherPlugin(t *testing.T) {
	for _, ignite := range []bool{true, false} {
		var burning effect.Effect
		g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 125}}}
		g.hook = func(g *triggerStage) error {
			g.world.Effects().Define("burning", effect.Spec{effect.Lasts(time.Hour)})
			burning = g.world.Effects().Named("burning")
			if ignite {
				if err := g.obey(rule.Then[world.Moving]("the bullet ignites", rule.Self(g.bullet), rule.Unless(burning, rule.Apply(burning)))); err != nil {
					return err
				}
			}
			return g.obey(rule.Then[collision.Meeting]("fire spreads", rule.Between(burning.Mark(), tag.Any), rule.ForOther(rule.Apply(burning))))
		}
		runTriggers(t, g, 700*time.Millisecond)
		g.each(burning, func(_ uid.UID64, _ float64, bullet bool, under bool) {
			if under != ignite {
				t.Errorf("ignited %v: the bullet %v is burning %v; want %v", ignite, bullet, under, ignite)
			}
		})
	}
}
