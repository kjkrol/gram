package main

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
)

// benchInit is a game.Initializer that drives the real Stage without a window;
// Scene.Layers() is left out.
type benchInit struct {
	hosts   []plugin.Host // of the rules of the moments the plugins installed catch
	ecs     *goke.ECS
	world   *world.Plugin
	tracked []any
	pending []func() []goke.System
	tps     game.TPS
}

var _ game.Initializer = (*benchInit)(nil)

func (c *benchInit) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.tracked = append(c.tracked, m)
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}

func (c *benchInit) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.tracked = append(c.tracked, p)
		c.pending = append(c.pending, p.SetupSystems)
	}
}

func (c *benchInit) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *benchInit) ECS() *goke.ECS                                  { return c.ecs }
func (c *benchInit) TPS() *game.TPS                                  { return &c.tps }

func (c *benchInit) Use(p plugin.Plugin) error {
	c.tracked = append(c.tracked, p)
	return p.Install(c)
}

func (c *benchInit) Track(s plugin.Serializable) error {
	c.tracked = append(c.tracked, s)
	return nil
}

// Screen is the window's size, which the cameras are sized to, as the engine's Initializer says.

func (c *benchInit) UseWorld(cfg world.Config) *world.Plugin {
	c.world = world.NewPlugin(cfg)
	c.tracked = append(c.tracked, c.world)
	if err := c.world.Install(c); err != nil {
		panic(err)
	}
	return c.world
}

// buildStage runs the fresh-spawn half of entering a Stage: Init, Spawn, Populate, Setup.
func buildStage(tb testing.TB) (*goke.ECS, *arena) {
	tb.Helper()

	rng = rand.New(rand.NewPCG(0x5eed, 0xc0ffee))

	a, stage := newArena()
	ctx := &benchInit{ecs: goke.New()}
	if err := stage.Init(ctx); err != nil {
		tb.Fatalf("Init: %v", err)
	}
	if err := ctx.Deliver(ctx.world.Kinds().Played()...); err != nil { // as the engine does once Init returns
		tb.Fatalf("roles: %v", err)
	}
	if err := stage.Spawn(); err != nil {
		tb.Fatalf("Spawn: %v", err)
	}
	for _, v := range ctx.tracked {
		if p, ok := v.(plugin.Populator); ok {
			if err := p.Populate(); err != nil {
				tb.Fatalf("Populate: %v", err)
			}
		}
	}
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { stage.Update(rc, d); a.world.Clock().Replay(rc, d) })

	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)

	return ctx.ecs, a
}

// benchStep is one tick at the demo's TPS.
const benchStep = time.Second / TPS

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *benchInit) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *benchInit) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
