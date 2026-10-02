package engine

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
)

// hookStage uses the plugins before, the world, the plugins after, seeds one walker and hooks
// rules through its Initializer, keeping what Hook said in hooked.
type hookStage struct {
	before, after []plugin.Plugin
	rules         []rule.Rule
	hooked        error
	ctx           game.Initializer // kept to hook once more after the Stage is set up

	world  *world.Plugin
	walker kind.Of[struct{}]
	base   goke.Comp[world.Base]
	query  *goke.Query
	stack  game.Scenes
}

func (s *hookStage) Name() string { return "stage" }

func (s *hookStage) Init(ctx game.Initializer) error {
	s.ctx = ctx
	for _, p := range s.before {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	s.world = ctx.UseWorld(testWorldConfig())
	for _, p := range s.after {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	ctx.Setup(s)
	s.walker = kind.Define[struct{}](s.world.Kinds(), "walker", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(50, 50), 5, 5)}),
		comp.Const(world.Velocity{}),
	})
	s.hooked = ctx.Hook(s.rules...)
	return nil
}

func (s *hookStage) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		s.query = si.NewQueryBuilder(&s.base).Build()
	}}}
}

func (s *hookStage) Restore(game.Persistence) (bool, error) { return false, nil }

func (s *hookStage) Spawn() error {
	s.world.Seed(s.walker.Entry(struct{}{}))
	return nil
}

func (s *hookStage) Update(ctx goke.RunCtx, d time.Duration) { s.world.RunPlan(ctx, d) }

func (s *hookStage) Stack() game.Scenes {
	if s.stack == nil {
		s.stack, _ = game.NewStack()
	}
	return s.stack
}

// living is how many entities with a world.Base the Stage's ECS holds.
func (s *hookStage) living() int {
	n := 0
	for s.query.All(); s.query.Next(); {
		n += len(s.query.Cursor().IDs)
	}
	return n
}

// runHookStage enters s and ticks its ECS twice at the engine's step, the walker counted after
// Init and after the ticks.
func runHookStage(t *testing.T, s *hookStage) (before, after int) {
	t.Helper()
	eng := NewEngine(oneStageGame{stage: s})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	before = s.living()
	for range 2 {
		eng.current.host.ecs.Tick(eng.step)
	}
	return before, s.living()
}

// despawning is a rule of world.Moving: every entity gives itself a world.Despawn.
func despawning() rule.Rule {
	return rule.On("leave", rule.All, func(m *rule.Moment[world.Moving]) rule.Step {
		return m.Order(world.Despawn{})
	})
}

// hostPlugin is a plugin hosting rules: it notes every rule it is asked to take and answers err.
type hostPlugin struct {
	stubPlugin
	err   error
	asked []rule.Rule
}

func (p *hostPlugin) Hook(rules ...rule.Rule) error {
	p.asked = append(p.asked, rules...)
	return p.err
}

var _ host = (*hostPlugin)(nil)

// unhostedMoment is a moment no plugin catches.
type unhostedMoment struct{}

func TestInitializer_Hook_WorldRunsTheRuleWithinTwoTicks(t *testing.T) {
	s := &hookStage{rules: []rule.Rule{despawning()}}
	before, after := runHookStage(t, s)

	if s.hooked != nil {
		t.Fatalf("Hook = %v, want the world to take a rule of world.Moving", s.hooked)
	}
	if before != 1 || after != 0 {
		t.Errorf("walkers = %d after Init, %d after two ticks; want 1, then 0 despawned by the rule", before, after)
	}
}

func TestInitializer_Hook_WithoutTheRuleTheWalkerStays(t *testing.T) {
	before, after := runHookStage(t, &hookStage{})

	if before != 1 || after != 1 {
		t.Errorf("walkers = %d after Init, %d after two ticks; want 1 and 1 with no rule hooked", before, after)
	}
}

func TestInitializer_Hook_UnhostedMomentFailsInit(t *testing.T) {
	nobody := rule.On("nobody", rule.All, func(m *rule.Moment[unhostedMoment]) rule.Step {
		return m.Order(world.Despawn{})
	})
	other := &stubPlugin{name: "test.plain"}
	eng := newTestEngine(func(ctx game.Initializer) error {
		if err := ctx.Use(other); err != nil {
			return err
		}
		ctx.UseWorld(testWorldConfig())
		return ctx.Hook(nobody)
	})

	err := eng.Init()
	if !errors.Is(err, plugin.ErrUnhosted) {
		t.Fatalf("Init() = %v, want an error wrapping plugin.ErrUnhosted", err)
	}
	if want := `"nobody" of engine.unhostedMoment`; !strings.Contains(err.Error(), want) {
		t.Errorf("Init() = %q, want it to name the rule (%s)", err, want)
	}
	if eng.current != nil {
		t.Errorf("the Stage was entered despite its Init failing")
	}
}

func TestInitializer_Hook_SkipsAHostRefusingTheMoment(t *testing.T) {
	refusing := &hostPlugin{stubPlugin: stubPlugin{name: "test.refusing"}, err: fmt.Errorf("%w: not mine", plugin.ErrUnhosted)}
	s := &hookStage{
		before: []plugin.Plugin{&stubPlugin{name: "test.plain"}, refusing},
		rules:  []rule.Rule{despawning()},
	}
	before, after := runHookStage(t, s)

	if s.hooked != nil {
		t.Fatalf("Hook = %v, want the world to take the rule the first host refused", s.hooked)
	}
	if len(refusing.asked) != 1 {
		t.Errorf("the host used before the world was asked %d times, want once", len(refusing.asked))
	}
	if before != 1 || after != 0 {
		t.Errorf("walkers = %d after Init, %d after two ticks; want 1, then 0 despawned by the world's rule", before, after)
	}
}

func TestInitializer_Hook_AnotherErrorStopsHook(t *testing.T) {
	broken := errors.New("test: host broken")
	failing := &hostPlugin{stubPlugin: stubPlugin{name: "test.failing"}, err: broken}
	taking := &hostPlugin{stubPlugin: stubPlugin{name: "test.taking"}}
	s := &hookStage{before: []plugin.Plugin{failing, taking}, rules: []rule.Rule{despawning()}}
	before, after := runHookStage(t, s)

	if !errors.Is(s.hooked, broken) {
		t.Fatalf("Hook = %v, want the failing host's error", s.hooked)
	}
	if len(failing.asked) != 1 || len(taking.asked) != 0 {
		t.Errorf("asked: failing %d, taking %d; want 1 and 0 — Hook stops at the first other error",
			len(failing.asked), len(taking.asked))
	}
	if before != 1 || after != 1 {
		t.Errorf("walkers = %d after Init, %d after two ticks; want 1 and 1 — the world never saw the rule", before, after)
	}
}

func TestInitializer_Hook_TriesHostsInUseOrder(t *testing.T) {
	t.Run("a host used before the world takes the rule from it", func(t *testing.T) {
		taking := &hostPlugin{stubPlugin: stubPlugin{name: "test.taking"}}
		s := &hookStage{before: []plugin.Plugin{taking}, rules: []rule.Rule{despawning()}}
		before, after := runHookStage(t, s)

		if s.hooked != nil {
			t.Fatalf("Hook = %v, want nil", s.hooked)
		}
		if len(taking.asked) != 1 {
			t.Errorf("the host used before the world was asked %d times, want once", len(taking.asked))
		}
		if before != 1 || after != 1 {
			t.Errorf("walkers = %d after Init, %d after two ticks; want 1 and 1 — the world never got the rule", before, after)
		}
	})
	t.Run("a host used after the world is not asked once the world takes it", func(t *testing.T) {
		taking := &hostPlugin{stubPlugin: stubPlugin{name: "test.taking"}}
		s := &hookStage{after: []plugin.Plugin{taking}, rules: []rule.Rule{despawning()}}
		before, after := runHookStage(t, s)

		if s.hooked != nil {
			t.Fatalf("Hook = %v, want nil", s.hooked)
		}
		if len(taking.asked) != 0 {
			t.Errorf("the host used after the world was asked %d times, want never", len(taking.asked))
		}
		if before != 1 || after != 0 {
			t.Errorf("walkers = %d after Init, %d after two ticks; want 1, then 0 despawned by the world's rule", before, after)
		}
	})
}

func TestInitializer_Hook_AfterSetupIsRefusedAsBuilt(t *testing.T) {
	s := &hookStage{}
	runHookStage(t, s)

	err := s.ctx.Hook(despawning())
	if !errors.Is(err, plugin.ErrHostBuilt) {
		t.Errorf("Hook after Setup = %v, want an error wrapping plugin.ErrHostBuilt", err)
	}
	if errors.Is(err, plugin.ErrUnhosted) {
		t.Errorf("Hook after Setup = %v, want it not to read as unhosted", err)
	}
}
