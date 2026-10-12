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
)

type treeStage struct {
	load  bool
	plan  *written
	world *world.Plugin
	unit  kind.Of[struct{}]
	probe *treeProbe
	stack game.Scenes
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
		sentry = write("sentry", rule.Steps(
			rule.Wait(10*time.Millisecond),
			rule.Wait(time.Hour),
			rule.Apply(world.Despawn{}),
		))
	}
	kind.Define[struct{}](g.world.Kinds(), "sentry", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		sentry.on(g.world),
	})
	g.unit = kind.Named[struct{}](g.world.Kinds(), "sentry")
	return nil
}

func (g *treeStage) Loaded() error { return nil }

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

func (g *treeStage) guarding(t *testing.T) (since time.Duration, running bool) {
	t.Helper()
	n, since, running := g.minds()
	if n != 1 {
		t.Fatalf("%d entities with a mind, want 1", n)
	}
	return since, running
}

func TestPlan_RunsInTheWorldAndIsSaved(t *testing.T) {
	t.Chdir(t.TempDir())
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
	if err := eng.Save(""); err != nil {
		t.Fatal(err)
	}
	loader := &treeStage{load: true}
	eng2 := engine.NewEngine(oneStageGame{stage: loader, props: game.Props{}})
	if err := eng2.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng2.Init(); err != nil {
		t.Fatal(err)
	}
	if at, running := loader.guarding(t); at != since || !running {
		t.Errorf("after a load the sentry's wait began at %v, its tree running %v; want %v, running", at, running, since)
	}
}

func TestPlan_OrdersTheWorldsCommands(t *testing.T) {
	stage := &treeStage{plan: write("leaver", rule.Steps(
		rule.Wait(50*time.Millisecond),
		rule.Apply(world.Despawn{}),
	))}
	eng := engine.NewEngine(oneStageGame{stage: stage, props: game.Props{}})
	if stage.load {
		if err := eng.Load("", ""); err != nil {
			t.Fatalf("Load: %v", err)
		}
	}
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

type written struct{ plan rule.Planned }

func write(name string, root rule.Step) *written { return &written{rule.Plan(name, root)} }

func (p *written) on(*world.Plugin) comp.Comp { return p.plan }
