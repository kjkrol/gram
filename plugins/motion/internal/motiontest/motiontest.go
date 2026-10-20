// Package motiontest is what motion's tests share: an installer and a world with motion, stepped
// as a game steps it.
package motiontest

import (
	"testing"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// InstallCtx is the plugin.Installer a Stage would hand over, minus the engine.
type InstallCtx struct {
	hosts   []host.Host
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *InstallCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *InstallCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *InstallCtx) RegSys(factory func() goke.System) goke.Runnable {
	return c.ecs.RegSys(factory())
}
func (c *InstallCtx) ECS() *goke.ECS { return c.ecs }

// Hosts keeps the hosts a plugin tells of.
func (c *InstallCtx) Hosts(h ...host.Host)   { c.hosts = append(c.hosts, h...) }
func (c *InstallCtx) Draws(...render.Source) {}
func (c *InstallCtx) Looks(func())           {}

// Start installs w and m on a new ECS, hands rules to the hosts of their moments, sets up their
// systems and after, and plans Step. Seed and populate before it.
func Start(
	t testing.TB,
	w *world.Plugin,
	m *motion.Plugin,
	rules []act.Rule,
	after ...goke.System,
) *goke.ECS {
	t.Helper()
	ctx := &InstallCtx{ecs: goke.New()}
	if err := w.Install(ctx); err != nil {
		t.Fatalf("world Install: %v", err)
	}
	if err := m.Install(ctx); err != nil {
		t.Fatalf("motion Install: %v", err)
	}
	if err := ceremony.Deliver(ctx.hosts, rules...); err != nil {
		t.Fatalf("rules: %v", err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(append(systems, after...)...)
	ctx.ecs.SetPlan(Step(w, m))
	return ctx.ecs
}

// Step is a step as a game takes it: the world, motion, the clock's replay.
func Step(w *world.Plugin, m *motion.Plugin) func(goke.RunCtx, time.Duration) {
	return func(rc goke.RunCtx, d time.Duration) {
		settling := w.Clock().Settling(rc)
		w.RunPlan(settling, d)
		m.RunPlan(settling, d)
		settling.Sync()
		w.Clock().Replay(rc, d)
	}
}
