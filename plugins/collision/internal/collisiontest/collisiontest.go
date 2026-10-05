// Package collisiontest is what the collision's tests share: an installer and a world with
// collision, stepped as a game steps it.
package collisiontest

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
)

// InstallCtx is the plugin.Installer a Stage would hand over, minus the engine.
type InstallCtx struct {
	hosts   []plugin.Host // of the rules of the moments the plugins installed catch
	ecs     *goke.ECS
	pending []func() []goke.System
	tracked []any
}

// NewInstallCtx is an installer onto ecs.
func NewInstallCtx(ecs *goke.ECS) *InstallCtx { return &InstallCtx{ecs: ecs} }

func (c *InstallCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.tracked = append(c.tracked, m)
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *InstallCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.tracked = append(c.tracked, p)
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *InstallCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *InstallCtx) ECS() *goke.ECS                                  { return c.ecs }

// Systems is what the installed plugins set up, in the order they were installed.
func (c *InstallCtx) Systems() []goke.System {
	var systems []goke.System
	for _, produce := range c.pending {
		systems = append(systems, produce()...)
	}
	return systems
}

// Tracked is every module and provider installed, for their components and PostLoad on a load.
func (c *InstallCtx) Tracked() []any { return c.tracked }

// Start installs w and c on a new ECS, sets their systems up and then each of after, and plans
// Step. Seed and populate before it.
func Start(t testing.TB, w *world.Plugin, c *collision.Plugin, after ...goke.System) *goke.ECS {
	t.Helper()
	return StartObeying(t, w, c, nil, after...)
}

// StartObeying is Start with rules handed to the hosts of their moments once the plugins are
// installed.
func StartObeying(t testing.TB, w *world.Plugin, c *collision.Plugin, rules []rule.Rule, after ...goke.System) *goke.ECS {
	t.Helper()
	ctx := NewInstallCtx(goke.New())
	if err := w.Install(ctx); err != nil {
		t.Fatalf("world Install: %v", err)
	}
	if err := c.Install(ctx); err != nil {
		t.Fatalf("collision Install: %v", err)
	}
	if err := ctx.Deliver(rules...); err != nil {
		t.Fatalf("rules: %v", err)
	}
	ctx.ecs.Setup(append(ctx.Systems(), after...)...)
	ctx.ecs.SetPlan(Step(w, c))
	return ctx.ecs
}

// Step is a step as a game takes it: the world, the collision, the clock's replay.
func Step(w *world.Plugin, c *collision.Plugin) func(goke.RunCtx, time.Duration) {
	return func(rc goke.RunCtx, d time.Duration) {
		w.RunPlan(rc, d)
		c.RunPlan(rc, d)
		rc.Sync()
		w.Clock().Replay(rc, d)
	}
}

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *InstallCtx) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *InstallCtx) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
