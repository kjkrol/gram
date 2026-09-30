package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func testWorldConfig() world.Config {
	return world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	}
}

type stubPlugin struct {
	name         string
	installed    int
	installFn    func(ctx plugin.Installer) error
	serializable plugin.Serializable
}

func (p *stubPlugin) Name() string { return p.name }
func (p *stubPlugin) Install(ctx plugin.Installer) error {
	p.installed++
	if p.installFn != nil {
		return p.installFn(ctx)
	}
	return nil
}
func (p *stubPlugin) RunPlan(goke.RunCtx, time.Duration) {}
func (p *stubPlugin) WithRenderer(render.AtlasSource)    {}
func (p *stubPlugin) Renderer() render.Layer             { return nil }
func (p *stubPlugin) EventHandler() control.EventHandler { return nil }
func (p *stubPlugin) Serializable() plugin.Serializable  { return p.serializable }
func (p *stubPlugin) Hook(...plugin.Trigger) error       { return plugin.ErrUnhosted }

// stubStage is a minimal game.Stage for testing Engine/Initializer.
type stubStage struct {
	name   string
	initFn func(ctx game.Initializer) error
	stack  game.Scenes
}

func (s *stubStage) Name() string {
	if s.name == "" {
		return "stage"
	}
	return s.name
}
func (s *stubStage) Init(ctx game.Initializer) error {
	if s.initFn != nil {
		return s.initFn(ctx)
	}
	return nil
}
func (s *stubStage) Restore(game.Persistence) (bool, error) { return false, nil }
func (s *stubStage) Spawn() error                           { return nil }
func (s *stubStage) Update(goke.RunCtx, time.Duration)      {}
func (s *stubStage) Stack() game.Scenes {
	if s.stack == nil {
		s.stack, _ = game.NewStack()
	}
	return s.stack
}

// oneStageGame is a minimal game.Game wrapping a single Stage — enough for
// tests that only care about Engine/Initializer behavior within one Stage.
type oneStageGame struct {
	stage game.Stage
	props game.Props
}

func (g oneStageGame) Props() game.Props { return g.props }

func (g oneStageGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

func newTestEngine(initFn func(ctx game.Initializer) error) *Engine {
	return NewEngine(oneStageGame{stage: &stubStage{initFn: initFn}})
}

func TestInitializer_Use_InstallsOnce(t *testing.T) {
	p := &stubPlugin{name: "test.plugin"}
	eng := newTestEngine(func(ctx game.Initializer) error { return ctx.Use(p) })

	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if p.installed != 1 {
		t.Errorf("installed = %d, want 1", p.installed)
	}
}

func TestInitializer_Use_DuplicateNameRejected(t *testing.T) {
	eng := newTestEngine(func(ctx game.Initializer) error {
		if err := ctx.Use(&stubPlugin{name: "test.plugin"}); err != nil {
			return err
		}
		return ctx.Use(&stubPlugin{name: "test.plugin"})
	})

	if err := eng.Init(); err == nil {
		t.Fatal("expected an error using a second plugin with the same Name")
	}
}

func TestEngine_Init_PropagatesGameInitError(t *testing.T) {
	wantErr := errors.New("test: Game.Init failed")
	eng := newTestEngine(func(ctx game.Initializer) error { return wantErr })

	if err := eng.Init(); err != wantErr {
		t.Fatalf("Init() = %v, want %v", err, wantErr)
	}
}

type stubSetupProvider struct {
	callCount int
	systems   []goke.System
}

func (s *stubSetupProvider) SetupSystems() []goke.System {
	s.callCount++
	return s.systems
}

func TestEngine_Init_EvaluatesSetupSystemsLazily(t *testing.T) {
	stub := &stubSetupProvider{}
	var duringInit int
	eng := newTestEngine(func(ctx game.Initializer) error {
		ctx.Setup(stub)
		duringInit = stub.callCount
		return nil
	})

	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if duringInit != 0 {
		t.Fatalf("SetupSystems called %d times during Stage.Init, want 0 (must stay lazy)", duringInit)
	}
	if stub.callCount != 1 {
		t.Errorf("SetupSystems called %d times after engine.Init(), want 1", stub.callCount)
	}
}

func TestInitializer_Use_RegistersSerializableByName(t *testing.T) {
	p := &stubPlugin{name: "test.resource", serializable: &testPersisted{N: 7}}
	eng := newTestEngine(func(ctx game.Initializer) error { return ctx.Use(p) })

	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	targets, ok := eng.current.host.resources.persisted()["test.resource"]
	if !ok || len(targets) != 1 || *(targets[0].(*int)) != 7 {
		t.Errorf("expected stubPlugin's Serializable() registered under its Name(), got %+v, ok=%v", targets, ok)
	}
}

type testPersisted struct{ N int }

func (p *testPersisted) Persisted() []any { return []any{&p.N} }

// stubBuiltinPlugin implements Builtin, so Use must reject it.
type stubBuiltinPlugin struct{ stubPlugin }

func (*stubBuiltinPlugin) Builtin() {}

func TestInitializer_Use_RejectsBuiltinPlugin(t *testing.T) {
	p := &stubBuiltinPlugin{stubPlugin: stubPlugin{name: "test.builtin"}}
	eng := newTestEngine(func(ctx game.Initializer) error { return ctx.Use(p) })

	if err := eng.Init(); err == nil {
		t.Fatal("expected Use to reject a Plugin implementing Builtin")
	}
	if p.installed != 0 {
		t.Errorf("installed = %d, want 0 (rejected before Install)", p.installed)
	}
}

func TestEngine_Init_WorldViewportDefaultsToScreenSize(t *testing.T) {
	props := game.Props{ScreenWidth: 200, ScreenHeight: 150}
	var got camera.Camera
	stage := &stubStage{initFn: func(ctx game.Initializer) error {
		got = ctx.UseWorld(world.Config{
			Space:    world.SpaceCfg{Width: 1000, Height: 1000},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		}).Camera()
		return nil
	}}
	eng := NewEngine(oneStageGame{stage: stage, props: props})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	b := got.Bounds()
	if w, h := b.BottomRight.X-b.TopLeft.X, b.BottomRight.Y-b.TopLeft.Y; w != 200 || h != 150 {
		t.Errorf("Camera().Bounds() size = %vx%v, want 200x150 (screen size, not world size)", w, h)
	}
}

func TestInitializer_UseWorld_ReturnsInstalledInstance(t *testing.T) {
	var got *world.Plugin
	eng := newTestEngine(func(ctx game.Initializer) error {
		got = ctx.UseWorld(testWorldConfig())
		return nil
	})

	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got == nil || eng.current.world != got {
		t.Fatalf("UseWorld() = %v, want the world.Plugin the engine installed for the Stage (%v)", got, eng.current.world)
	}
}

func TestInitializer_UseWorld_SecondCallPanics(t *testing.T) {
	eng := newTestEngine(func(ctx game.Initializer) error {
		ctx.UseWorld(testWorldConfig())
		defer func() {
			if recover() == nil {
				t.Error("expected a second UseWorld in the same Stage to panic")
			}
		}()
		ctx.UseWorld(testWorldConfig())
		return nil
	})

	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
}

func TestEngine_Init_StageWithoutUseWorldGetsNoWorld(t *testing.T) {
	eng := newTestEngine(func(game.Initializer) error { return nil })

	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if eng.current.world != nil {
		t.Errorf("Stage world = %v, want nil when the Stage never calls UseWorld", eng.current.world)
	}
	if cam := eng.Camera(); cam != nil {
		t.Errorf("Engine.Camera() = %v, want nil without a world", cam)
	}
}
