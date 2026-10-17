package motion_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/internal/motiontest"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
)

const (
	keptRole  = "kept"
	thingKind = "thing"
)

type motionStage struct {
	moving bool // motion made and used

	world  *world.Plugin
	motion *motion.Plugin
	query  *goke.Query
	base   goke.Comp[world.Base]
	course goke.OptComp[steering.Course]
	stack  game.Scenes
}

func (g *motionStage) Name() string { return "stage" }

func (g *motionStage) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000, Edges: aabbworld.OpenX},
		Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 10, MaxSize: 10},
	})
	if g.moving {
		g.motion = motion.NewPlugin(g.world)
	}
	g.world.Roles().Define(keptRole,
		act.Branch[motion.Leaving]{
			Name: "kept",
			When: cond.Self.If(cond.Always),
			Then: act.Self.Do(steering.Turn{}),
		},
		act.Branch[motion.Moving]{
			Name: "turning",
			When: cond.Self.If(motion.Still()),
			Then: act.Self.Do(steering.Turn{}),
		})
	kind.Define[struct{}](g.world.Kinds(), thingKind, kind.Spec{
		comp.Const(box(985, 500, 10)),
		comp.Const(world.Velocity{Dir: geom.NewVec(1, 0), Value: 300}),
		comp.Const(steering.Course{WantSpeed: 7}),
		ceremony.Plays(g.world.Roles().Named(keptRole)),
	})
	ctx.Setup(g)
	if g.moving {
		return ctx.Use(g.motion)
	}
	return nil
}

func (g *motionStage) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		g.query = si.NewQueryBuilder(&g.base).Optional(&g.course).
			Include(goke.Include[motion.Outside]()).Build()
	}}}
}

func (g *motionStage) Loaded() error { return nil }

func (g *motionStage) Spawn() error {
	g.world.Seed(kind.Named[struct{}](g.world.Kinds(), thingKind).Entry(struct{}{}))
	return nil
}

func (g *motionStage) Update(ctx goke.RunCtx, d time.Duration) {
	g.world.RunPlan(ctx, d)
	g.motion.RunPlan(ctx, d)
}

func (g *motionStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

type oneStageGame struct{ stage game.Stage }

func (g oneStageGame) Props() game.Props { return game.Props{} }

func (g oneStageGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

// the course of every thing marked Outside
func (g *motionStage) leavers() []steering.Course {
	var out []steering.Course
	for g.query.All(); g.query.Next(); {
		out = append(out, g.course.Slice(g.query.Cursor())...)
	}
	return out
}

func TestMotion_HostsTheRulesOfItsMoments(t *testing.T) {
	err := engine.NewEngine(oneStageGame{&motionStage{}}).Init()
	if !errors.Is(err, host.ErrUnhosted) {
		t.Errorf("a Stage without motion obeying rules of its moments: Init %v, want unhosted", err)
	}
	if err := engine.NewEngine(oneStageGame{&motionStage{moving: true}}).Init(); err != nil {
		t.Errorf("a Stage with motion: Init %v, want its moments hosted", err)
	}
}

func TestMotion_KeepsALeaversMarkAndCourseOverASaveAndALoad(t *testing.T) {
	t.Chdir(t.TempDir())
	saver := &motionStage{moving: true}
	eng := engine.NewEngine(oneStageGame{saver})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	for end := time.Now().Add(300 * time.Millisecond); time.Now().Before(end); {
		time.Sleep(5 * time.Millisecond)
		if err := eng.Update(); err != nil {
			t.Fatal(err)
		}
	}
	if got := saver.leavers(); len(got) != 1 {
		t.Fatalf("%d leavers before saving, want the one kept by its rule", len(got))
	}
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loader := &motionStage{moving: true}
	again := engine.NewEngine(oneStageGame{loader})
	if err := again.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := again.Init(); err != nil {
		t.Fatal(err)
	}
	got := loader.leavers()
	if len(got) != 1 || got[0].WantSpeed != 7 {
		t.Errorf("leavers after the load %+v, want the one still Outside, asked for 7", got)
	}
}

func TestUnitTemplate_CarriesACourseAndTheWorldsEntityDefaults(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	motion.NewPlugin(w)

	spec := w.Roster().Spec(motion.UnitTemplate, comp.Const(world.Position{}))

	got := map[string]bool{}
	for _, c := range spec {
		got[fmt.Sprintf("%v", c.Type())] = true
	}
	for _, want := range []string{
		"steering.Course",
		"entity.Velocity",
		"entity.Shadow",
		"tag.Tags[github.com/kjkrol/ceremony/effect.States]",
		"entity.Position",
	} {
		if !got[want] {
			t.Errorf("a unit's spec carries %v, want a %s", got, want)
		}
	}
}

func TestMotion_AnEffectOnTheSteeringTellsTheStepItBegins(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	m := motion.NewPlugin(w)
	fx := w.Effects()
	fx.Define("held", effect.Spec{
		effect.Lasts(time.Second),
		effect.Alter(func(s *steering.Steering) { s.Halted = true }),
	})
	kind.Define[thing](w.Kinds(), thingKind, kind.Spec{
		comp.Const(box(500, 500, 10)),
		comp.Const(world.Velocity{Dir: east, Value: 100}),
		comp.Const(steering.Steering{MaxSpeed: 100}),
		comp.Const(steering.Course{Want: east, Speed: 100, WantSpeed: 100}),
	})
	w.Seed(kind.Named[thing](w.Kinds(), thingKind).Entry(thing{}))
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var all *goke.Query
	var base goke.Comp[world.Base]
	var steer goke.Comp[steering.Steering]
	ecs := motiontest.Start(t, w, m, nil, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		all = si.NewQueryBuilder(&base, &steer).Build()
	}})
	cast := false
	caster := ecs.RegSys(goke.SystemFn{OnUpdate: func(*goke.CmdBuf, time.Duration) {
		for all.All(); all.Next() && !cast; cast = true {
			fx.Cast(all.Cursor().IDs[0], fx.Named("held"))
		}
	}})
	step := motiontest.Step(w, m)
	ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		rc.Run(caster, d)
		rc.Sync()
		step(rc, d)
	})

	was := 500.0
	for tick := 1; tick <= 4; tick++ {
		ecs.Tick(time.Second / 60)
		for all.All(); all.Next(); {
			cur := all.Cursor()
			got := base.Slice(cur)[0]
			if steer.Slice(cur)[0].Halted {
				if got.Pos.TopLeft.X != was || got.Vel.Value != 0 {
					t.Errorf("tick %d, the step it was held: from %v to %v going %v, want it "+
						"to stand", tick, was, got.Pos.TopLeft.X, got.Vel.Value)
				}
				return
			}
			was = got.Pos.TopLeft.X
		}
	}
	t.Fatal("never held")
}
