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

// hookStage uses the plugins before, the world, the plugins after, and seeds one walker playing
// roles: the engine hands their rules to the plugins' hosts once Init returns.
type hookStage struct {
	plays         []role // the roles the walker plays, defined in the Stage's world
	before, after []plugin.Plugin

	world  *world.Plugin
	walker kind.Of[struct{}]
	base   goke.Comp[world.Base]
	query  *goke.Query
	stack  game.Scenes
}

func (s *hookStage) Name() string { return "stage" }

func (s *hookStage) Init(ctx game.Initializer) error {
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
	spec := kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(50, 50), 5, 5)}),
		comp.Const(world.Velocity{}),
	}
	if s.plays != nil {
		var parts []*rule.Part
		for _, r := range s.plays {
			s.world.Roles().Define(r.name, r.rules...)
			parts = append(parts, s.world.Roles().Named(r.name))
		}
		spec = append(spec, rule.Plays(parts...))
	}
	kind.Define[struct{}](s.world.Kinds(), "walker", spec)
	s.walker = kind.Named[struct{}](s.world.Kinds(), "walker")
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
	return rule.Then[world.Moving]("leave", rule.All, rule.Order(world.Despawn{}))
}

// hostPlugin is a plugin whose host of rules notes every rule it is asked to take and answers err.
type hostPlugin struct {
	stubPlugin
	err   error
	asked []any
}

func (p *hostPlugin) Install(ctx plugin.Installer) error {
	ctx.Hosts(p)
	return nil
}

func (p *hostPlugin) Add(r any) error {
	p.asked = append(p.asked, r)
	return p.err
}

// unhostedMoment is a moment no plugin catches.
type unhostedMoment struct{}

// role is a role for the Stage to define: its name and the rules its players obey.
type role struct {
	name  string
	rules []rule.Rule
}

// leaver is a role whose players despawn.
func leaver() role { return role{"leaver", []rule.Rule{despawning()}} }

// A role a kind plays reaches the host of its rule's moment once Init returns: the rule runs
// though the Stage handed nothing over.
func TestEngine_DeliversTheRolesItsKindsPlay(t *testing.T) {
	before, after := runHookStage(t, &hookStage{plays: []role{leaver()}})

	if before != 1 || after != 0 {
		t.Errorf("walkers = %d after Init, %d after two ticks; want 1 then 0", before, after)
	}
}

// The role's rule is for its players alone: a walker playing another role, or none, stays.
func TestEngine_ARolesRuleIsForItsPlayersAlone(t *testing.T) {
	for name, plays := range map[string][]role{
		"playing another": {{name: "hook immortal"}},
		"playing none":    nil,
	} {
		if before, after := runHookStage(t, &hookStage{plays: plays}); before != 1 || after != 1 {
			t.Errorf("%s: walkers = %d after Init, %d after two ticks; want 1 and 1", name, before, after)
		}
	}
}

// A played role whose rule no plugin in use catches fails the Stage's Init, naming the rule and
// its role.
func TestEngine_RefusesAPlayedRoleNobodyHosts(t *testing.T) {
	lost := role{"lost soul", []rule.Rule{rule.Then[unhostedMoment]("lost", rule.All, rule.Order(world.Despawn{}))}}
	eng := NewEngine(oneStageGame{stage: &hookStage{before: []plugin.Plugin{&stubPlugin{name: "test.plain"}}, plays: []role{lost}}})
	err := eng.Init()
	if !errors.Is(err, plugin.ErrUnhosted) {
		t.Fatalf("Init = %v, want plugin.ErrUnhosted", err)
	}
	for _, want := range []string{`"lost" of engine.unhostedMoment`, "for the role lost soul"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Init = %q, want it to name %s", err, want)
		}
	}
	if eng.current != nil {
		t.Errorf("the Stage was entered despite its Init failing")
	}
}

// The hosts are tried in the order their plugins were used, the next while one refuses the
// moment; any other error stops the delivery and fails Init.
func TestEngine_TriesTheHostsInUseOrder(t *testing.T) {
	t.Run("a host refusing the moment is skipped", func(t *testing.T) {
		refusing := &hostPlugin{stubPlugin: stubPlugin{name: "test.refusing"}, err: fmt.Errorf("%w: not mine", plugin.ErrUnhosted)}
		s := &hookStage{before: []plugin.Plugin{&stubPlugin{name: "test.plain"}, refusing}, plays: []role{leaver()}}
		before, after := runHookStage(t, s)
		if len(refusing.asked) != 1 {
			t.Errorf("the host used before the world was asked %d times, want once", len(refusing.asked))
		}
		if before != 1 || after != 0 {
			t.Errorf("walkers = %d after Init, %d after two ticks; want 1, then 0 despawned by the world's rule", before, after)
		}
	})
	t.Run("a host used before the world takes the rule from it", func(t *testing.T) {
		taking := &hostPlugin{stubPlugin: stubPlugin{name: "test.taking"}}
		before, after := runHookStage(t, &hookStage{before: []plugin.Plugin{taking}, plays: []role{leaver()}})
		if len(taking.asked) != 1 {
			t.Errorf("the host used before the world was asked %d times, want once", len(taking.asked))
		}
		if before != 1 || after != 1 {
			t.Errorf("walkers = %d after Init, %d after two ticks; want 1 and 1 — the world never got the rule", before, after)
		}
	})
	t.Run("a host used after the world is not asked once the world takes it", func(t *testing.T) {
		taking := &hostPlugin{stubPlugin: stubPlugin{name: "test.taking"}}
		before, after := runHookStage(t, &hookStage{after: []plugin.Plugin{taking}, plays: []role{leaver()}})
		if len(taking.asked) != 0 {
			t.Errorf("the host used after the world was asked %d times, want never", len(taking.asked))
		}
		if before != 1 || after != 0 {
			t.Errorf("walkers = %d after Init, %d after two ticks; want 1, then 0 despawned by the world's rule", before, after)
		}
	})
	t.Run("another error stops the delivery", func(t *testing.T) {
		broken := errors.New("test: host broken")
		failing := &hostPlugin{stubPlugin: stubPlugin{name: "test.failing"}, err: broken}
		taking := &hostPlugin{stubPlugin: stubPlugin{name: "test.taking"}}
		s := &hookStage{before: []plugin.Plugin{failing, taking}, plays: []role{leaver()}}
		if err := NewEngine(oneStageGame{stage: s}).Init(); !errors.Is(err, broken) {
			t.Fatalf("Init = %v, want the failing host's error", err)
		}
		if len(failing.asked) != 1 || len(taking.asked) != 0 {
			t.Errorf("asked: failing %d, taking %d; want 1 and 0", len(failing.asked), len(taking.asked))
		}
	})
}
