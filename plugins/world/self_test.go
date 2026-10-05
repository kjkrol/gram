package world_test

import (
	"fmt"
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

// dial is a knob of the made-up plugin of these tests.
type dial struct{ Level int }

// skyStage is a world with a plugin's own entity carrying a dial, a storm that turns the dial up,
// ten plain states and one scout, lit while the sky is under the storm.
type skyStage struct {
	loadFrom string

	world  *world.Plugin
	sky    *world.Self
	ecs    *goke.ECS
	storm  effect.Effect
	lit    effect.Effect
	states []effect.Effect
	scout  kind.Of[struct{}]
	dials  goke.Comp[dial]
	dialed *goke.Query
	base   goke.Comp[world.Base]
	units  *goke.Query
	stack  game.Scenes
}

func (g *skyStage) Name() string { return "stage" }

func (g *skyStage) Init(ctx game.Initializer) error {
	g.world, g.ecs = ctx.UseWorld(testWorldConfig()), ctx.ECS()
	g.sky = world.NewSelf(g.world, "test.sky", comp.Const(dial{Level: 1}))
	ctx.Setup(g)
	fx := g.world.Effects()
	fx.Define("storm", effect.Spec{effect.Lasts(time.Hour), effect.Alter(func(d *dial) { d.Level = 5 })})
	g.storm = fx.Named("storm")
	fx.Define("lit", effect.Spec{})
	g.lit = fx.Named("lit")
	for i := range 10 {
		name := fmt.Sprintf("state %d", i)
		fx.Define(name, effect.Spec{effect.Lasts(time.Hour)})
		g.states = append(g.states, fx.Named(name))
	}
	g.world.Roles().Define("sky watcher",
		rule.Then[world.Moving]("lit by the storm", rule.All, rule.While(g.sky, g.storm, rule.Keep(g.lit))))
	watcher := g.world.Roles().Named("sky watcher")
	kind.Define[struct{}](g.world.Kinds(), "scout", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		rule.Plays(watcher),
	})
	g.scout = kind.Named[struct{}](g.world.Kinds(), "scout")
	return nil
}

func (g *skyStage) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		g.dialed = si.NewQueryBuilder(&g.dials).Build()
		g.units = si.NewQueryBuilder(&g.base).Build()
	}}}
}

func (g *skyStage) Restore(p game.Persistence) (bool, error) {
	if g.loadFrom == "" {
		return false, nil
	}
	return true, p.Load(g.loadFrom, "")
}

func (g *skyStage) Spawn() error {
	g.world.Seed(g.scout.Entry(struct{}{}).Named("scout"))
	return nil
}

func (g *skyStage) Update(ctx goke.RunCtx, d time.Duration) { g.world.RunPlan(ctx, d) }

func (g *skyStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

// give gives cmds as nobody and ticks them through.
func (g *skyStage) give(t *testing.T, cmds ...any) {
	t.Helper()
	for _, cmd := range cmds {
		if !g.world.Carrier().Put(control.Nobody, cmd) {
			t.Fatalf("the world carries no %T", cmd)
		}
	}
	g.ecs.Tick(time.Second / 60)
	g.ecs.Tick(time.Second / 60)
}

// dialsOf are the dials in the world, by entity.
func (g *skyStage) dialsOf() map[uid.UID64]int {
	out := map[uid.UID64]int{}
	for g.dialed.All(); g.dialed.Next(); {
		cur := g.dialed.Cursor()
		for i, id := range cur.IDs {
			out[id] = g.dials.Slice(cur)[i].Level
		}
	}
	return out
}

// theScout is the one unit.
func (g *skyStage) theScout(t *testing.T) uid.UID64 {
	t.Helper()
	for g.units.All(); g.units.Next(); {
		return g.units.Cursor().IDs[0]
	}
	t.Fatal("no scout")
	return 0
}

func runSky(t *testing.T, g *skyStage) *engine.Engine {
	t.Helper()
	eng := engine.NewEngine(oneStageGame{stage: g, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return eng
}

// A plugin's own entity carries its knob; a command for the plugin puts an effect on that entity,
// whose Alter turns the knob and gives it back, the plugin told its entity Changed; and a rule's
// While runs while the plugin is under the effect.
func TestSelf_AnEffectOnThePluginTurnsItsKnob(t *testing.T) {
	g := &skyStage{}
	runSky(t, g)
	sky := g.sky.Entity()
	if got := g.dialsOf(); len(got) != 1 || got[sky] != 1 {
		t.Fatalf("dials %v, want the one of the plugin's entity %d at 1", got, sky)
	}
	scout := g.theScout(t)

	if !g.world.Carrier().Put(control.Nobody, rule.Cast(g.storm).On(g.sky)) {
		t.Fatal("the world carries no command for a plugin")
	}
	g.ecs.Tick(time.Second / 60)
	if !g.sky.Changed() {
		t.Error("the plugin's entity is not Changed in the step the storm turned its knob")
	}
	g.ecs.Tick(time.Second / 60)
	if g.sky.Changed() {
		t.Error("the plugin's entity is still Changed a step on")
	}
	if got := g.dialsOf()[sky]; got != 5 || !g.storm.On(sky) || g.storm.On(g.world.Entity()) {
		t.Errorf("under the storm the dial is %d, the plugin under it %v, the world %v; want 5, true, false", got, g.storm.On(sky), g.storm.On(g.world.Entity()))
	}
	if !g.lit.On(scout) {
		t.Error("the scout is not lit While the sky is under the storm")
	}

	g.give(t, rule.Lift(g.storm).On(g.sky))
	if got := g.dialsOf()[sky]; got != 1 || g.storm.On(sky) {
		t.Errorf("the storm lifted, the dial is %d, want the 1 it was", got)
	}
}

// The world is a plugin as any: its own entity is the clock's, entity.World and the plugin itself
// name it alike.
func TestSelf_TheWorldsOwnEntityIsTheClocks(t *testing.T) {
	g := &skyStage{}
	runSky(t, g)
	if g.world.Entity() != g.world.Clock().Entity() {
		t.Fatalf("the world's entity is %d, the clock's %d; want one", g.world.Entity(), g.world.Clock().Entity())
	}
	g.give(t, rule.Cast(g.storm).On(g.world))
	if !g.storm.On(g.world.Clock().Entity()) {
		t.Error("a command for the world plugin did not reach the clock's entity")
	}
	g.give(t, rule.Lift(g.storm).On(entity.World))
	if g.storm.On(g.world.Entity()) {
		t.Error("a command for entity.World did not reach the world's own entity")
	}
}

// A plugin's entity holds every effect at once, where a unit holds eight.
func TestSelf_HoldsMoreEffectsAtOnceThanAUnit(t *testing.T) {
	g := &skyStage{}
	runSky(t, g)
	var cmds []any
	for _, e := range g.states {
		cmds = append(cmds, rule.Cast(e).On(g.sky), rule.Cast(e).On(entity.Named("scout")))
	}
	g.give(t, cmds...)
	onSky, onScout := 0, 0
	scout := g.theScout(t)
	for _, e := range g.states {
		if e.On(g.sky.Entity()) {
			onSky++
		}
		if e.On(scout) {
			onScout++
		}
	}
	if onSky != len(g.states) || onScout != 8 {
		t.Errorf("of %d states the plugin is under %d and the scout under %d; want all and 8", len(g.states), onSky, onScout)
	}
}

// A loaded game brings the plugins' entities, found again by their names: none is made twice, and
// the knob and the effect on it are as they were saved.
func TestSelf_IsFoundAgainAfterALoad(t *testing.T) {
	path := t.TempDir() + "/save"
	saver := &skyStage{}
	eng := runSky(t, saver)
	saver.give(t, rule.Cast(saver.storm).On(saver.sky))
	if err := eng.Persistence().Save(path, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	g := &skyStage{loadFrom: path}
	runSky(t, g)
	sky := g.sky.Entity()
	if got := g.dialsOf(); len(got) != 1 || got[sky] != 5 {
		t.Fatalf("after the load the dials are %v, want the one of the plugin's entity %d, at 5 under the storm", got, sky)
	}
	if g.world.Entity() != g.world.Clock().Entity() {
		t.Errorf("after the load the world's entity is %d, the clock's %d; want one", g.world.Entity(), g.world.Clock().Entity())
	}
	g.give(t, rule.Lift(g.storm).On(g.sky))
	if got := g.dialsOf()[sky]; got != 1 {
		t.Errorf("the storm lifted after the load, the dial is %d, want the 1 it was before the storm", got)
	}
}
