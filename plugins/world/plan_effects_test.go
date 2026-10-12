package world_test

import (
	"github.com/kjkrol/gram/internal/steps"
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
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

type effectStage struct {
	plan  func(fx *effect.Effects) *written
	world *world.Plugin
	unit  kind.Of[struct{}]
	marks goke.OptComp[tag.Tags[effect.States]]
	tally goke.Comp[tally]
	mind  goke.Comp[steps.Mind]
	query *goke.Query
	stack game.Scenes
}

func (g *effectStage) Name() string { return "stage" }

func (g *effectStage) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(testWorldConfig())
	ctx.Setup(effectProbe{g})
	kind.Define[struct{}](g.world.Kinds(), "glower", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		comp.Const(tally{}),
		g.plan(g.world.Effects()).on(g.world),
	})
	g.unit = kind.Named[struct{}](g.world.Kinds(), "glower")
	return nil
}

type effectProbe struct{ g *effectStage }

func (p effectProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.g.query = si.NewQueryBuilder(&p.g.mind, &p.g.tally).Optional(&p.g.marks).Build()
	}}}
}

func (g *effectStage) Loaded() error                           { return nil }
func (g *effectStage) Spawn() error                            { g.world.Seed(g.unit.Entry(struct{}{})); return nil }
func (g *effectStage) Update(ctx goke.RunCtx, d time.Duration) { g.world.RunPlan(ctx, d) }
func (g *effectStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

type tally struct{ N int }

var counting = effect.Alter(func(t *tally) { t.N++ })

func (g *effectStage) under(e effect.Effect) bool {
	for g.query.All(); g.query.Next(); {
		if m := g.marks.Slice(g.query.Cursor()); m != nil {
			return m[0].Has(e.Mark())
		}
	}
	return false
}

func (g *effectStage) tallied() int {
	for g.query.All(); g.query.Next(); {
		return g.tally.Slice(g.query.Cursor())[0].N
	}
	return 0
}

func run(t *testing.T, g *effectStage) *engine.Engine {
	t.Helper()
	e := engine.NewEngine(oneStageGame{stage: g, props: game.Props{}})
	if err := e.Init(); err != nil {
		t.Fatal(err)
	}
	return e
}

func step(t *testing.T, e *engine.Engine, d time.Duration) {
	t.Helper()
	for end := time.Now().Add(d); time.Now().Before(end); {
		time.Sleep(5 * time.Millisecond)
		if err := e.Update(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlan_KeepHoldsAnEffectAsLongAsItsBranchRuns(t *testing.T) {
	var held effect.Effect
	g := &effectStage{plan: func(fx *effect.Effects) *written {
		fx.Define("held", effect.Spec{})
		held = fx.Named("held")
		return write("hold a while", rule.OneOf(
			rule.Branch[any]{Name: "hold", On: rule.Self, Do: rule.Hold(held)}.
				Timeout(300*time.Millisecond).
				Every(time.Hour),
			rule.Branch[any]{Name: "rest", On: rule.Self, Do: rule.Idle()},
		))
	}}
	e := run(t, g)
	step(t, e, 150*time.Millisecond)
	if !g.under(held) {
		t.Fatal("while its branch runs the entity is not under the effect")
	}
	step(t, e, 400*time.Millisecond)
	if g.under(held) {
		t.Error("its branch over, the entity is still under the effect")
	}
}

func TestPlan_OnceKeepsItsMemoryInAnEffect(t *testing.T) {
	var marked, tallying effect.Effect
	g := &effectStage{plan: func(fx *effect.Effects) *written {
		fx.Define("marked", effect.Spec{effect.Lasts(400 * time.Millisecond)})
		marked = fx.Named("marked")
		fx.Define("tally", effect.Spec{effect.Lasts(time.Hour), effect.Stacking(), counting})
		tallying = fx.Named("tally")
		return write("tally once a while",
			rule.OneOf(
				rule.Branch[any]{
					Name: "tally",
					When: rule.Not(rule.Under(marked)),
					On:   rule.Self,
					Do:   rule.Steps(rule.Put(marked), rule.Put(tallying)),
				},
				rule.Branch[any]{Name: "idle", On: rule.Self, Do: rule.Idle()},
			))
	}}
	e := run(t, g)
	step(t, e, time.Second)
	if n := g.tallied(); n < 2 || n > 4 {
		t.Errorf("in a second the tally stacked %d times, want about once in 400 ms: 2 to 4", n)
	}
}
