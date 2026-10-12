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

type roles struct{}

type spot struct {
	x, vx  float64
	bullet bool
}

type triggerStage struct {
	spots          []spot
	hook           func(g *triggerStage) error
	rules          []rule.Rule
	world          *world.Plugin
	coll           *collision.Plugin
	bullet, target *rule.Part
	unit           kind.Of[spot]
	base           goke.Comp[world.Base]
	states         goke.OptComp[tag.Tags[effect.States]]
	tallies        goke.OptComp[tally]
	marks          goke.OptComp[tag.Tags[rule.Roles]]
	query          *goke.Query
	stack          game.Scenes
}

func (g *triggerStage) Name() string { return "stage" }

func (g *triggerStage) obey(rules ...rule.Rule) error {
	g.rules = append(g.rules, rules...)
	return nil
}

func (g *triggerStage) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	g.world.Roles().Define("bullet")
	g.world.Roles().Define("target")
	g.bullet, g.target = g.world.Roles().Named("bullet"), g.world.Roles().Named("target")
	ctx.Setup(triggerProbe{g})
	g.coll = collision.NewPlugin(g.world)
	if err := g.hook(g); err != nil {
		return err
	}
	g.world.Roles().Define("trigger unit", g.rules...)
	kind.Define[spot](g.world.Kinds(), "unit", kind.Spec{
		comp.Load(func(s spot) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(s.x, 100), 10, 10)}
		}),
		comp.Load(func(s spot) world.Velocity { return world.Velocity{Dir: geom.NewVec(1, 0), Value: s.vx} }),
		comp.Const(collision.Collider{}),
		comp.Const(tally{}),
		comp.Load(func(s spot) tag.Tags[rule.Roles] {
			trigger := g.world.Roles().Named("trigger unit")
			if s.bullet {
				return rule.Plays(trigger, g.bullet).Resolve(nil)
			}
			return rule.Plays(trigger, g.target).Resolve(nil)
		}),
	})
	g.unit = kind.Named[spot](g.world.Kinds(), "unit")
	return ctx.Use(g.coll)
}

type triggerProbe struct{ g *triggerStage }

func (p triggerProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.g.query = si.NewQueryBuilder(&p.g.base).Optional(&p.g.states, &p.g.tallies, &p.g.marks).Build()
	}}}
}

func (g *triggerStage) Loaded() error { return nil }
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

func (g *triggerStage) each(e effect.Effect, fn func(id uid.UID64, x float64, bullet bool, under bool)) {
	for g.query.All(); g.query.Next(); {
		cur := g.query.Cursor()
		states, marks := g.states.Slice(cur), g.marks.Slice(cur)
		for i, id := range cur.IDs {
			under := e != (effect.Effect{}) && states != nil && states[i].Has(e.Mark())
			bullet := marks != nil && marks[i].Has(g.bullet.Tag())
			fn(id, g.base.Slice(cur)[i].Pos.TopLeft.X, bullet, under)
		}
	}
}

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

func TestRule_PlayingNarrowsToTheRolesPlayers(t *testing.T) {
	var marked effect.Effect
	g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 500, vx: 60}}}
	g.hook = func(g *triggerStage) error {
		g.world.Effects().Define("marked", effect.Spec{effect.Lasts(time.Hour)})
		marked = g.world.Effects().Named("marked")
		return g.obey(rule.Branch[world.Moving]{
			Name: "mark the bullet",
			On:   rule.Self,
			When: rule.Playing(g.bullet),
			Do:   rule.Put(marked),
		})
	}
	runTriggers(t, g, 300*time.Millisecond)
	g.each(marked, func(_ uid.UID64, _ float64, bullet bool, under bool) {
		if under != bullet {
			t.Errorf("the bullet %v is marked %v; want the bullet marked alone", bullet, under)
		}
	})
}

func TestRule_OnOtherActsOnWhomTheEntityMet(t *testing.T) {
	var marked effect.Effect
	g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 125}}}
	g.hook = func(g *triggerStage) error {
		g.world.Effects().Define("marked", effect.Spec{effect.Lasts(time.Hour)})
		marked = g.world.Effects().Named("marked")
		return g.obey(rule.Branch[collision.Meeting]{
			Name: "mark the target",
			On:   rule.Other,
			When: rule.All(rule.Playing(g.bullet), rule.Paired(g.target)),
			Do:   rule.Put(marked),
		})
	}
	runTriggers(t, g, 700*time.Millisecond)
	g.each(marked, func(_ uid.UID64, _ float64, bullet bool, under bool) {
		if under == bullet {
			t.Errorf("the bullet %v is marked %v; want the target marked alone", bullet, under)
		}
	})
}

func TestRule_AnEffectIsItsMemory(t *testing.T) {
	var mark, tallying effect.Effect
	g := &triggerStage{spots: []spot{{x: 100, vx: 1}}}
	g.hook = func(g *triggerStage) error {
		g.world.Effects().Define("mark", effect.Spec{effect.Lasts(400 * time.Millisecond)})
		mark = g.world.Effects().Named("mark")
		g.world.Effects().Define("tally", effect.Spec{effect.Lasts(time.Hour), effect.Stacking(), counting})
		tallying = g.world.Effects().Named("tally")
		return g.obey(rule.Branch[world.Moving]{
			Name: "once a while",
			On:   rule.Self,
			When: rule.Not(rule.Under(mark)),
			Do:   rule.Steps(rule.Put(mark), rule.Put(tallying)),
		})
	}
	runTriggers(t, g, time.Second)
	if count := g.tallied(); count < 2 || count > 4 {
		t.Errorf("in a second the guarded body ran %d times, want about once in 400 ms: 2 to 4", count)
	}
}

func TestRule_OrdersCommands(t *testing.T) {
	g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 500, vx: 60}}}
	g.hook = func(g *triggerStage) error {
		return g.obey(rule.Branch[world.Moving]{
			Name: "the bullet leaves",
			On:   rule.Self,
			When: rule.Playing(g.bullet),
			Do:   rule.Apply(world.Despawn{}),
		})
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

func TestRule_AnEffectsMarkerFiltersInAnotherPlugin(t *testing.T) {
	for _, ignite := range []bool{true, false} {
		var burning effect.Effect
		g := &triggerStage{spots: []spot{{x: 100, vx: 60, bullet: true}, {x: 125}}}
		g.hook = func(g *triggerStage) error {
			g.world.Effects().Define("burning", effect.Spec{effect.Lasts(time.Hour)})
			burning = g.world.Effects().Named("burning")
			if ignite {
				if err := g.obey(rule.Branch[world.Moving]{
					Name: "the bullet ignites",
					On:   rule.Self,
					When: rule.All(rule.Playing(g.bullet), rule.Not(rule.Under(burning))),
					Do:   rule.Put(burning),
				}); err != nil {
					return err
				}
			}
			return g.obey(rule.Branch[collision.Meeting]{
				Name: "fire spreads",
				On:   rule.Other,
				When: rule.Under(burning),
				Do:   rule.Put(burning),
			})
		}
		runTriggers(t, g, 700*time.Millisecond)
		g.each(burning, func(_ uid.UID64, _ float64, bullet bool, under bool) {
			if under != ignite {
				t.Errorf("ignited %v: the bullet %v is burning %v; want %v", ignite, bullet, under, ignite)
			}
		})
	}
}
