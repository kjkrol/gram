package world_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/gram/plugins/world/act/effect"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
)

// effectStage spawns one entity whose tree tree builds from the world's effects.
type effectStage struct {
	tree   func(fx *effect.Effects) act.Node
	world  *world.Plugin
	unit   kind.Of[struct{}]
	active goke.OptComp[effect.Active]
	mind   goke.Comp[act.Mind]
	query  *goke.Query
	stack  game.Scenes
}

func (g *effectStage) Name() string { return "stage" }

func (g *effectStage) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(testWorldConfig())
	ctx.Setup(effectProbe{g})
	g.unit = kind.Define[struct{}](g.world.Kinds(), "glower", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		act.Tree(g.tree(g.world.Effects())),
	})
	return nil
}

type effectProbe struct{ g *effectStage }

func (p effectProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.g.query = si.NewQueryBuilder(&p.g.mind).Optional(&p.g.active).Build()
	}}}
}

func (g *effectStage) Restore(game.Persistence) (bool, error)  { return false, nil }
func (g *effectStage) Spawn() error                            { g.world.Seed(g.unit.Entry(struct{}{})); return nil }
func (g *effectStage) Update(ctx goke.RunCtx, d time.Duration) { g.world.RunPlan(ctx, d) }
func (g *effectStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

// under counts the slots of e the stage's one entity holds.
func (g *effectStage) under(e effect.Effect) int {
	n := 0
	for g.query.All(); g.query.Next(); {
		if as := g.active.Slice(g.query.Cursor()); as != nil {
			for _, s := range as[0].Slots {
				if s.State != effect.Empty && s.Kind == e.ID() {
					n++
				}
			}
		}
	}
	return n
}

// run starts the stage's engine.
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

// While holds an effect as long as its branch runs, and takes it off when the branch gives way.
func TestTree_WhileHoldsAnEffectAsLongAsItsBranchRuns(t *testing.T) {
	var held effect.Effect
	g := &effectStage{tree: func(fx *effect.Effects) act.Node {
		held = fx.Define("held", effect.Spec{})
		c := act.Named("hold a while")
		return c.Do(c.Then(
			c.Invert(c.Timeout(300*time.Millisecond, c.While(held))),
			c.Wait(time.Hour)))
	}}
	e := run(t, g)
	step(t, e, 150*time.Millisecond)
	if g.under(held) != 1 {
		t.Fatalf("while its branch runs the entity holds %d of the effect, want 1", g.under(held))
	}
	step(t, e, 400*time.Millisecond)
	if n := g.under(held); n != 0 {
		t.Errorf("its branch over, the entity still holds %d of the effect, want none", n)
	}
}

// Unless keeps a memory in an effect: the tally is added only while the mark is not on, so at
// most once a mark's while.
func TestTree_UnlessKeepsItsMemoryInAnEffect(t *testing.T) {
	var marked, tally effect.Effect
	g := &effectStage{tree: func(fx *effect.Effects) act.Node {
		marked = fx.Define("marked", effect.Spec{effect.Lasts(400 * time.Millisecond)})
		tally = fx.Define("tally", effect.Spec{effect.Lasts(time.Hour), effect.Stacking()})
		c := act.Named("tally once a while")
		return c.Do(c.First(
			c.Unless(marked, c.Then(c.Apply(marked), c.Apply(tally))),
			c.Idle()))
	}}
	e := run(t, g)
	step(t, e, time.Second)
	if n := g.under(tally); n < 2 || n > 4 {
		t.Errorf("in a second the tally stacked %d times, want about once in 400 ms: 2 to 4", n)
	}
}
