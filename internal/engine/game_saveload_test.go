package engine_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
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

// ecsAccessor captures ctx.ECS() during Install and queues an optional setup callback.
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
func (a *ecsAccessor) WithRenderer(render.AtlasSource)    {}
func (a *ecsAccessor) Renderer() render.Layer             { return nil }
func (a *ecsAccessor) EventHandler() control.EventHandler { return nil }
func (a *ecsAccessor) Serializable() plugin.Serializable  { return nil }
func (a *ecsAccessor) Hook(...plugin.Rule) error          { return plugin.ErrUnhosted }

// saveLoadTestGame wires newTestWorldPlugin + ecsAccessor for the round-trip test below.
type saveLoadTestGame struct {
	acc      *ecsAccessor
	setup    func(*goke.SysInit)
	define   func(*world.Kinds) []kind.Entry
	world    *world.Plugin
	loadFrom string
	loadArgs []any

	stack game.Scenes
}

func (g *saveLoadTestGame) Name() string { return "stage" }
func (g *saveLoadTestGame) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(testWorldConfig())
	if g.define != nil {
		g.world.Seed(g.define(g.world.Kinds())...)
	}
	g.acc = &ecsAccessor{setup: g.setup}
	return ctx.Use(g.acc)
}
func (g *saveLoadTestGame) Restore(p game.Persistence) (bool, error) {
	if g.loadFrom == "" {
		return false, nil
	}
	if err := p.Load(g.loadFrom, "", g.loadArgs...); err != nil {
		return false, err
	}
	return true, nil
}
func (g *saveLoadTestGame) Spawn() error                      { return nil }
func (g *saveLoadTestGame) Update(goke.RunCtx, time.Duration) {}
func (g *saveLoadTestGame) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

// oneStageGame is a minimal game.Game wrapping a single Stage.
type oneStageGame struct {
	stage game.Stage
	props game.Props
}

func (g oneStageGame) Props() game.Props { return g.props }

func (g oneStageGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

func TestGame_SaveLoad_RoundTrip(t *testing.T) {
	basePath := t.TempDir() + "/save"

	var appearance goke.Comp[world.Appearance]
	g := &saveLoadTestGame{setup: func(si *goke.SysInit) {
		f := si.NewFactory(&appearance)
		f.Create(1)
		f.Next()
		appearance.Slice(&f.Cursor)[0] = world.Appearance{SpriteID: 7}
	}}
	eng := engine.NewEngine(oneStageGame{stage: g, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	state := &saveTestState{N: 42}
	extra := &saveTestResourceB{S: "hello"}
	if err := eng.Persistence().Save(basePath, "", state, extra); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var appearance2 goke.Comp[world.Appearance]
	var q *goke.Query
	state2 := &saveTestState{}
	extra2 := &saveTestResourceB{}
	game2 := &saveLoadTestGame{
		setup: func(si *goke.SysInit) {
			q = si.NewQueryBuilder(&appearance2).Build()
		},
		loadFrom: basePath,
		loadArgs: []any{state2, extra2},
	}
	eng2 := engine.NewEngine(oneStageGame{stage: game2, props: game.Props{}})
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
		t.Fatal("expected the saved world.Appearance entity to survive the round trip")
	}
}

type saveTestTag struct{}

type saveTestMark struct{ Left int }

func TestGame_SaveLoad_KeepsWhatAKindGivesItsEntities(t *testing.T) {
	basePath := t.TempDir() + "/save"
	define := func(kinds *world.Kinds) []kind.Entry {
		marked := kind.Define[struct{}](kinds, "marked", kind.Spec{
			comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
			comp.Const(world.Velocity{}),
			comp.Const(saveTestTag{}),
			comp.Const(saveTestMark{Left: 3}),
			comp.Const(steering.Steering{TurnRate: 0.5}),
		})
		return []kind.Entry{marked.Entry(struct{}{})}
	}

	eng := engine.NewEngine(oneStageGame{stage: &saveLoadTestGame{define: define}, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := eng.Persistence().Save(basePath, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var mark goke.Comp[saveTestMark]
	var q *goke.Query
	loaded := &saveLoadTestGame{
		define:   define,
		setup:    func(si *goke.SysInit) { q = si.NewQueryBuilder(&mark).Include(goke.Include[saveTestTag]()).Build() },
		loadFrom: basePath,
	}
	eng2 := engine.NewEngine(oneStageGame{stage: loaded, props: game.Props{}})
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
