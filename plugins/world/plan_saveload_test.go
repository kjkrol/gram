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
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/plan"
)

// treeStage spawns one unit under plan — a sentry's when nil, coming to its post, waiting an hour
// there, then gone — or loads it from loadFrom.
type treeStage struct {
	loadFrom string
	plan     *written
	world    *world.Plugin
	unit     kind.Of[struct{}]
	probe    *treeProbe
	stack    game.Scenes
}

type treeProbe struct {
	mind  goke.Comp[steps.Mind]
	query *goke.Query
}

func (p *treeProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.query = si.NewQueryBuilder(&p.mind).Build()
	}}}
}

func (g *treeStage) Name() string { return "stage" }

func (g *treeStage) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(testWorldConfig())
	g.probe = &treeProbe{}
	ctx.Setup(g.probe)
	sentry := g.plan
	if sentry == nil {
		sentry = write("sentry", func(a *plan.Actor) rule.Step {
			return a.Steps(a.Wait(10*time.Millisecond), a.Wait(time.Hour), a.Order(world.Despawn{}))
		})
	}
	g.unit = kind.Define[struct{}](g.world.Kinds(), "sentry", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		sentry.on(g.world),
	})
	return nil
}

func (g *treeStage) Restore(p game.Persistence) (bool, error) {
	if g.loadFrom == "" {
		return false, nil
	}
	return true, p.Load(g.loadFrom, "")
}

func (g *treeStage) Spawn() error {
	g.world.Seed(g.unit.Entry(struct{}{}))
	return nil
}

func (g *treeStage) Update(ctx goke.RunCtx, d time.Duration) { g.world.RunPlan(ctx, d) }

func (g *treeStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

// minds is how many entities have a mind, and when the tree of the last began its second wait, on
// the world's clock, and whether it runs.
func (g *treeStage) minds() (n int, since time.Duration, running bool) {
	for g.probe.query.All(); g.probe.query.Next(); {
		for _, m := range g.probe.mind.Slice(g.probe.query.Cursor()) {
			n++
			running = m.Running != [len(m.Running)]uint64{}
			since = m.Since[2]
		}
	}
	return n, since, running
}

// guarding is when the stage's one unit began its wait, and whether its tree runs.
func (g *treeStage) guarding(t *testing.T) (since time.Duration, running bool) {
	t.Helper()
	n, since, running := g.minds()
	if n != 1 {
		t.Fatalf("%d entities with a mind, want 1", n)
	}
	return since, running
}

// The world runs an entity's tree in its simulation: the wait in progress, the tree's own state,
// is saved with the game and goes on after a load.
func TestPlan_RunsInTheWorldAndIsSaved(t *testing.T) {
	path := t.TempDir() + "/save"
	saver := &treeStage{}
	eng := engine.NewEngine(oneStageGame{stage: saver, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	for range 6 {
		if err := eng.Update(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	since, running := saver.guarding(t)
	if since == 0 || !running {
		t.Fatalf("the sentry's wait at its post began at %v, its tree running %v; want begun, running", since, running)
	}
	if err := eng.Persistence().Save(path, ""); err != nil {
		t.Fatal(err)
	}
	loader := &treeStage{loadFrom: path}
	if err := engine.NewEngine(oneStageGame{stage: loader, props: game.Props{}}).Init(); err != nil {
		t.Fatal(err)
	}
	if at, running := loader.guarding(t); at != since || !running {
		t.Errorf("after a load the sentry's wait began at %v, its tree running %v; want %v, running", at, running, since)
	}
}

// A plan orders commands as a player does: the world's own Despawn takes the entity that gives it
// itself out of the world.
func TestPlan_OrdersTheWorldsCommands(t *testing.T) {
	stage := &treeStage{plan: write("leaver", func(a *plan.Actor) rule.Step {
		return a.Steps(a.Wait(50*time.Millisecond), a.Order(world.Despawn{}))
	})}
	eng := engine.NewEngine(oneStageGame{stage: stage, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	for range 10 {
		if err := eng.Update(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if n, _, _ := stage.minds(); n != 0 {
		t.Errorf("%d entities with a mind after the leaver's wait, want none: it despawned itself", n)
	}
}

// written is a plan as a test writes it, for the Stage's world to define (world.Plans).
type written struct {
	name string
	body func(a *plan.Actor) rule.Step
}

func write(name string, body func(a *plan.Actor) rule.Step) *written { return &written{name, body} }

// on defines the plan in w and hands back the component of an entity following it.
func (p *written) on(w *world.Plugin) comp.Comp {
	w.Plans().Define(p.name, p.body)
	return w.Plans().Named(p.name)
}
