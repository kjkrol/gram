package world_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// guardStage is three units in a row — a captain in the guards, a guard, a passer-by — and an
// effect to cast on them by what they are called.
type guardStage struct {
	loadFrom string

	world *world.Plugin
	ecs   *goke.ECS
	unit  kind.Of[float64]
	alert effect.Effect
	base  goke.Comp[world.Base]
	query *goke.Query
	stack game.Scenes
}

func (g *guardStage) Name() string { return "stage" }

func (g *guardStage) Init(ctx game.Initializer) error {
	cfg := testWorldConfig()
	cfg.Entities.MaxCount = 3
	g.world, g.ecs = ctx.UseWorld(cfg), ctx.ECS()
	ctx.Setup(g)
	g.alert = g.world.Effects().Define("alert", effect.Spec{effect.Lasts(time.Hour)})
	g.unit = kind.Define[float64](g.world.Kinds(), "unit", kind.Spec{
		comp.Load(func(x float64) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
	})
	return ctx.Commands(rule.Cast(g.alert).On(entity.Named("captain")), rule.Cast(g.alert).On(entity.Group("guards")))
}

func (g *guardStage) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) { g.query = si.NewQueryBuilder(&g.base).Build() }}}
}

func (g *guardStage) Restore(p game.Persistence) (bool, error) {
	if g.loadFrom == "" {
		return false, nil
	}
	return true, p.Load(g.loadFrom, "")
}

func (g *guardStage) Spawn() error {
	g.world.Seed(
		g.unit.Entry(100).Named("captain").InGroup("guards"),
		g.unit.Entry(200).InGroup("guards"),
		g.unit.Entry(300),
	)
	return nil
}

func (g *guardStage) Update(ctx goke.RunCtx, d time.Duration) { g.world.RunPlan(ctx, d) }

func (g *guardStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

// give gives cmd as nobody and ticks it through.
func (g *guardStage) give(t *testing.T, cmd any) {
	t.Helper()
	if !g.world.Commands().Put(control.Nobody, cmd) {
		t.Fatalf("the world carries no %T", cmd)
	}
	for range 2 {
		g.ecs.Tick(time.Second / 60)
	}
}

// alerted are the x of the units under alert.
func (g *guardStage) alerted() map[float64]bool {
	on := map[float64]bool{}
	for g.query.All(); g.query.Next(); {
		cur := g.query.Cursor()
		for i, id := range cur.IDs {
			if g.alert.On(uid.UID64(id)) {
				on[g.base.Slice(cur)[i].Pos.TopLeft.X] = true
			}
		}
	}
	return on
}

// A command reaches the unit that bears the name it says, the units in the group it says, or the
// world itself, and nobody else.
func TestCasting_ReachesThoseNamedGroupedAndTheWorld(t *testing.T) {
	g := &guardStage{}
	runGuards(t, g)

	g.give(t, rule.Cast(g.alert).On(entity.Named("captain")))
	if on := g.alerted(); len(on) != 1 || !on[100] {
		t.Fatalf("alert the captain: units at %v alerted, want the one at 100 alone", on)
	}
	g.give(t, rule.Lift(g.alert).On(entity.Named("captain")))
	g.give(t, rule.Cast(g.alert).On(entity.Group("guards")))
	if on := g.alerted(); len(on) != 2 || !on[100] || !on[200] {
		t.Fatalf("alert the guards: units at %v alerted, want those at 100 and 200", on)
	}
	g.give(t, rule.Lift(g.alert).On(entity.Group("guards")))
	g.give(t, rule.Cast(g.alert).On(entity.World))
	if on := g.alerted(); len(on) != 0 || !g.alert.On(g.world.Clock().Entity()) {
		t.Errorf("alert the world: units at %v alerted, the world %v; want the world alone", on, g.alert.On(g.world.Clock().Entity()))
	}
}

// What the units are called is saved with them: a loaded game's captain is still the one found.
func TestCasting_NamesSurviveASaveAndALoad(t *testing.T) {
	path := t.TempDir() + "/save"
	if err := runGuards(t, &guardStage{}).Persistence().Save(path, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	g := &guardStage{loadFrom: path}
	runGuards(t, g)
	g.give(t, rule.Cast(g.alert).On(entity.Group("guards")))
	if on := g.alerted(); len(on) != 2 || !on[100] || !on[200] {
		t.Errorf("after a load, alert the guards: units at %v alerted, want those at 100 and 200", on)
	}
}

// A command that names nobody is refused as it is handed over.
func TestCommands_RefuseACommandForNobody(t *testing.T) {
	w := world.NewPlugin(testWorldConfig())
	if err := w.Triggers(rule.Cast(w.Effects().Define("alert", effect.Spec{}))); err == nil {
		t.Error("a Cast with no On was taken, want an error")
	}
}

// runGuards builds the stage through the engine: Init, Restore or Spawn, Setup.
func runGuards(t *testing.T, g *guardStage) *engine.Engine {
	t.Helper()
	eng := engine.NewEngine(oneStageGame{stage: g, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return eng
}
