package engine_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func testWorldConfig() world.Config {
	return world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 10, MinSize: 1, MaxSize: 100},
	}
}

type saveTestState struct{ N int }
type saveTestResourceB struct{ S string }

func (s *saveTestState) Persisted() []any     { return []any{&s.N} }
func (r *saveTestResourceB) Persisted() []any { return []any{&r.S} }

type ecsAccessor struct {
	ecs   *goke.ECS
	setup func(*goke.SysInit)
}

func (a *ecsAccessor) Name() string { return "test.ecs-accessor" }
func (a *ecsAccessor) Install(ctx plugin.Installer) error {
	a.ecs = ctx.ECS()
	if a.setup != nil {
		ctx.Setup(a)
	}
	return nil
}
func (a *ecsAccessor) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: a.setup}}
}
func (a *ecsAccessor) RunPlan(goke.RunCtx, time.Duration) {}
func (a *ecsAccessor) EventHandler() control.EventHandler { return nil }
func (a *ecsAccessor) Serializable() plugin.Serializable  { return nil }

type saveLoadTestGame struct {
	acc     *ecsAccessor
	setup   func(*goke.SysInit)
	define  func(*world.Kinds) []kind.Entry
	world   *world.Plugin
	tracked []plugin.Serializable // the game's own state, saved beside the plugins'

	stack game.Scenes
}

func (g *saveLoadTestGame) Name() string { return "stage" }
func (g *saveLoadTestGame) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(testWorldConfig())
	if g.define != nil {
		g.world.Seed(g.define(g.world.Kinds())...)
	}
	for _, s := range g.tracked {
		if err := ctx.Track(s); err != nil {
			return err
		}
	}
	g.acc = &ecsAccessor{setup: g.setup}
	return ctx.Use(g.acc)
}
func (g *saveLoadTestGame) Loaded() error                     { return nil }
func (g *saveLoadTestGame) Spawn() error                      { return nil }
func (g *saveLoadTestGame) Update(goke.RunCtx, time.Duration) {}
func (g *saveLoadTestGame) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

type oneStageGame struct {
	stage game.Stage
	props game.Props
}

func (g oneStageGame) Props() game.Props { return g.props }

func (g oneStageGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

func TestGame_SaveLoad_RoundTrip(t *testing.T) {
	props := game.Props{Saves: t.TempDir()}

	var appearance goke.Comp[render.Appearance]
	state := &saveTestState{N: 42}
	extra := &saveTestResourceB{S: "hello"}
	g := &saveLoadTestGame{tracked: []plugin.Serializable{state, extra}, setup: func(si *goke.SysInit) {
		f := si.NewFactory(&appearance)
		f.Create(1)
		f.Next()
		appearance.Slice(&f.Cursor)[0] = render.Appearance{SpriteID: 7}
	}}
	eng := engine.NewEngine(oneStageGame{stage: g, props: props})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var appearance2 goke.Comp[render.Appearance]
	var q *goke.Query
	state2 := &saveTestState{}
	extra2 := &saveTestResourceB{}
	game2 := &saveLoadTestGame{
		setup: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&appearance2).Build()
		},
		tracked: []plugin.Serializable{state2, extra2},
	}
	eng2 := engine.NewEngine(oneStageGame{stage: game2, props: props})
	if err := eng2.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if state2.N != 42 {
		t.Errorf("state2.N after Load = %d, want 42", state2.N)
	}
	if extra2.S != "hello" {
		t.Errorf("extra2.S after Load = %q, want %q", extra2.S, "hello")
	}

	q.All()
	found := false
	for q.Next() {
		for _, a := range appearance2.Slice(q.Cursor()) {
			found = true
			if a.SpriteID != 7 {
				t.Errorf("SpriteID = %d, want 7", a.SpriteID)
			}
		}
	}
	if !found {
		t.Fatal("expected the saved render.Appearance entity to survive the round trip")
	}
}

type saveTestTag struct{}

type saveTestMark struct{ Left int }

func TestGame_SaveLoad_KeepsWhatAKindGivesItsEntities(t *testing.T) {
	props := game.Props{Saves: t.TempDir()}
	define := func(kinds *world.Kinds) []kind.Entry {
		kind.Define[struct{}](kinds, "marked", kind.Spec{
			comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
			comp.Const(world.Velocity{}),
			comp.Const(saveTestTag{}),
			comp.Const(saveTestMark{Left: 3}),
			comp.Const(steering.Steering{TurnRate: 0.5}),
		})
		marked := kind.Named[struct{}](kinds, "marked")
		return []kind.Entry{marked.Entry(struct{}{})}
	}

	eng := engine.NewEngine(oneStageGame{stage: &saveLoadTestGame{define: define}, props: props})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var mark goke.Comp[saveTestMark]
	var q *goke.Query
	loaded := &saveLoadTestGame{
		define: define,
		setup:  func(si *goke.SysInit) { q = si.NewQueryBuilder(&mark).Include(goke.Include[saveTestTag]()).Build() },
	}
	eng2 := engine.NewEngine(oneStageGame{stage: loaded, props: props})
	if err := eng2.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init from the save: %v", err)
	}

	var marks []saveTestMark
	for q.All(); q.Next(); {
		marks = append(marks, mark.Slice(q.Cursor())...)
	}
	if len(marks) != 1 || marks[0].Left != 3 {
		t.Errorf("tagged entities after Load = %+v, want the one saved with Left 3", marks)
	}
}
