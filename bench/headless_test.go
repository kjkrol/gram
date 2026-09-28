// Package bench_test measures gram's per-tick costs through the same public API a game uses,
// on a Stage built without a window: plugins are installed through a headless game.Initializer
// and ticked directly on their ECS.
package bench_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
)

// headless is the least of a game.Initializer that installs plugins with no window: it queues
// what Install asks for and hands it all to one ecs.Setup, as the engine does after Stage.Init.
type headless struct {
	ecs     *goke.ECS
	world   *world.Plugin
	tracked []any
	pending []func() []goke.System
	tps     game.TPS
}

var _ game.Initializer = (*headless)(nil)

func newHeadless() *headless { return &headless{ecs: goke.New()} }

func (c *headless) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.tracked = append(c.tracked, m)
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}

func (c *headless) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.tracked = append(c.tracked, p)
		c.pending = append(c.pending, p.SetupSystems)
	}
}

func (c *headless) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *headless) ECS() *goke.ECS                                  { return c.ecs }
func (c *headless) TPS() *game.TPS                                  { return &c.tps }

func (c *headless) Use(p plugin.Plugin) error {
	c.tracked = append(c.tracked, p)
	return p.Install(c)
}

func (c *headless) Track(s plugin.Serializable) error {
	c.tracked = append(c.tracked, s)
	return nil
}

func (c *headless) UseWorld(cfg world.Config) *world.Plugin {
	if cfg.Camera.ViewportWidth == 0 {
		cfg.Camera.ViewportWidth, cfg.Camera.ViewportHeight = cfg.Space.Width, cfg.Space.Height
	}
	c.world = world.NewPlugin(cfg)
	c.tracked = append(c.tracked, c.world)
	if err := c.world.Install(c); err != nil {
		panic(err)
	}
	return c.world
}

// start runs the fresh-spawn half of entering a Stage after Init and Spawn: Populate on every
// tracked Populator, then one ecs.Setup, with plan as the tick.
func (c *headless) start(tb testing.TB, plan func(goke.RunCtx, time.Duration)) *goke.ECS {
	tb.Helper()
	for _, v := range c.tracked {
		if p, ok := v.(plugin.Populator); ok {
			if err := p.Populate(); err != nil {
				tb.Fatalf("Populate: %v", err)
			}
		}
	}
	c.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		plan(rc, d)
		if c.world != nil {
			c.world.Clock().Replay(rc, d) // as the engine does after the game's Update: run what the plan simulates
		}
	})
	var systems []goke.System
	for _, produce := range c.pending {
		systems = append(systems, produce()...)
	}
	c.ecs.Setup(systems...)
	return c.ecs
}

// step is one tick at 60 TPS.
const step = time.Second / 60

func entities(n int) string { return fmt.Sprintf("entities=%d", n) }
