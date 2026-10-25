package world_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type guardStage struct {
	load bool

	world *world.Plugin
	ecs   *goke.ECS
	unit  kind.Of[float32]
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
	g.world.Write().Effects().Define("alert", effect.Spec{effect.Lasts(time.Hour)})
	g.alert = g.world.Write().Effects().Named("alert")
	kind.Define[float32](g.world.Kinds(), "unit", kind.Spec{
		comp.Load(func(x float32) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
	})
	g.unit = kind.Named[float32](g.world.Kinds(), "unit")
	g.world.Commands().Define("alert the captain", ceremony.Put(g.alert).On(ceremony.Named("captain")))
	g.world.Commands().Define("alert the guards", ceremony.Put(g.alert).On(ceremony.Group("guards")))
	return nil
}

func (g *guardStage) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) { g.query = si.NewQueryBuilder(&g.base).Build() }}}
}

func (g *guardStage) Loaded() error { return nil }

func (g *guardStage) Spawn() error {
	g.world.Write().Seed(
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

func (g *guardStage) give(t *testing.T, cmd command.Command) {
	t.Helper()
	if !g.world.Write().Carrier().Put(command.Nobody, cmd) {
		t.Fatalf("the world carries no %T", cmd)
	}
	for range 2 {
		g.ecs.Tick(time.Second / 60)
	}
}

func (g *guardStage) alerted() map[float32]bool {
	on := map[float32]bool{}
	for g.query.All(); g.query.Next(); {
		cur := g.query.Cursor()
		for i, id := range cur.IDs {
			if g.world.Read(0).Effects().Has(uid.UID64(id), g.alert) {
				on[g.base.Slice(cur)[i].Pos.TopLeft.X] = true
			}
		}
	}
	return on
}

func TestCommand_ReachesThoseNamedGroupedAndTheWorld(t *testing.T) {
	g := &guardStage{}
	runGuards(t, g)

	g.give(t, ceremony.Put(g.alert).On(ceremony.Named("captain")))
	if on := g.alerted(); len(on) != 1 || !on[100] {
		t.Fatalf("alert the captain: units at %v alerted, want the one at 100 alone", on)
	}
	g.give(t, ceremony.Take(g.alert).On(ceremony.Named("captain")))
	g.give(t, ceremony.Put(g.alert).On(ceremony.Group("guards")))
	if on := g.alerted(); len(on) != 2 || !on[100] || !on[200] {
		t.Fatalf("alert the guards: units at %v alerted, want those at 100 and 200", on)
	}
	g.give(t, ceremony.Take(g.alert).On(ceremony.Group("guards")))
	g.give(t, ceremony.Put(g.alert).On(ceremony.World))
	read := g.world.Read(0)
	if on := g.alerted(); len(on) != 0 || !read.Effects().Has(read.Clock().Entity(), g.alert) {
		t.Errorf("alert the world: units at %v alerted, the world %v; want the world alone", on,
			read.Effects().Has(read.Clock().Entity(), g.alert))
	}
}

func TestCommand_ReachesTheEntityOfAnID(t *testing.T) {
	g := &guardStage{}
	runGuards(t, g)
	var passerBy uid.UID64
	for g.query.All(); g.query.Next(); {
		cur := g.query.Cursor()
		for i, id := range cur.IDs {
			if g.base.Slice(cur)[i].Pos.TopLeft.X == 300 {
				passerBy = id
			}
		}
	}
	g.give(t, ceremony.Put(g.alert).On(ceremony.ID(passerBy)))
	if on := g.alerted(); len(on) != 1 || !on[300] {
		t.Fatalf("alert the passer-by by its ID: units at %v alerted, want the one at 300 alone", on)
	}
}

func TestCommand_NamesSurviveASaveAndALoad(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := runGuards(t, &guardStage{}).Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	g := &guardStage{load: true}
	runGuards(t, g)
	g.give(t, ceremony.Put(g.alert).On(ceremony.Group("guards")))
	if on := g.alerted(); len(on) != 2 || !on[100] || !on[200] {
		t.Errorf("after a load, alert the guards: units at %v alerted, want those at 100 and 200", on)
	}
}

func TestCommands_AreDefinedOnceAndFoundByName(t *testing.T) {
	w := world.NewPlugin(testWorldConfig())
	w.Write().Effects().Define("alert", effect.Spec{})
	alert := w.Write().Effects().Named("alert")
	w.Commands().Define("alarm", ceremony.Put(alert).On(ceremony.World).For(time.Minute))
	if got := w.Commands().Named("alarm"); got.Effect != alert || got.Lasts != time.Minute || !got.Whom.(ceremony.Whom).IsWorld() {
		t.Errorf("the command found is %+v, want the one defined", got)
	}
	for name, c := range map[string]struct {
		do   func()
		want string
	}{
		"for nobody": {func() { w.Commands().Define("idle", ceremony.Put(alert)) }, "names nobody"},
		"defined twice": {
			func() { w.Commands().Define("alarm", ceremony.Take(alert).On(ceremony.World)) },
			`"alarm" is defined already`},
		"unknown": {func() { w.Commands().Named("retreat") }, `no command is defined as "retreat"`},
		"no name": {
			func() { w.Commands().Define("", ceremony.Put(alert).On(ceremony.World)) },
			"needs a name"},
	} {
		if msg := panicMessage(t, c.do); !strings.Contains(msg, c.want) {
			t.Errorf("%s: panic %q, want it to say %s", name, msg, c.want)
		}
	}
}

func runGuards(t *testing.T, g *guardStage) *engine.Engine {
	t.Helper()
	eng := engine.NewEngine(oneStageGame{stage: g, props: game.Props{}})
	if g.load {
		if err := eng.Load("", ""); err != nil {
			t.Fatalf("Load: %v", err)
		}
	}
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return eng
}
