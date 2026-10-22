// Package bench_test measures gram's per-tick costs through the public API a game uses, on a
// headless game.Initializer ticked directly on its ECS.
package bench_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type headless struct {
	hosts   []host.Host
	ecs     *goke.ECS
	world   *world.Plugin
	cam     camera.Camera
	tracked []any
	pending []func() []goke.System
	tps     game.TPS
	drawn   []render.Source
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
	c.world = world.NewPlugin(cfg)
	c.tracked = append(c.tracked, c.world)
	if err := c.world.Install(c); err != nil {
		panic(err)
	}
	return c.world
}

// w, h 0: the whole world
func (c *headless) camera(cfg cameras.Config, start cameras.Start, w, h float32) camera.Camera {
	if w == 0 {
		space := c.world.Res.Config.Space
		w, h = float32(space.Width), float32(space.Height)
	}
	cam := cameras.NewPlugin(c.world, cfg).New(start, camera.Config{})
	cam.SetViewport(w, h)
	cam.MoveTo(0, 0)
	return cam
}

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
		if c.world == nil {
			plan(rc, d)
			return
		}
		// as the engine runs the game's Update and the clock's replay after it
		clk := c.world.Clock()
		plan(clk.Settling(rc), d)
		clk.Replay(rc, d)
	})
	var systems []goke.System
	for _, produce := range c.pending {
		systems = append(systems, produce()...)
	}
	c.ecs.Setup(systems...)
	return c.ecs
}

const step = time.Second / 60

func entities(n int) string { return fmt.Sprintf("entities=%d", n) }

func (c *headless) Hosts(h ...host.Host)     { c.hosts = append(c.hosts, h...) }
func (c *headless) Draws(s ...render.Source) { c.drawn = append(c.drawn, s...) }
func (c *headless) Looks(func())             {}
func (*headless) Pool() *parallel.Pool       { return pool }

// the game's pool, every CPU, as the engine hands it to the plugins
var pool = parallel.New(0)

// Drawn is what the installed plugins draw, for the world's picture.
func (c *headless) Drawn() []render.Source { return c.drawn }

// drawnAs is the source of type T the installed plugins draw.
func drawnAs[T render.Source](b testing.TB, c *headless) T {
	b.Helper()
	for _, s := range c.drawn {
		if t, ok := s.(T); ok {
			return t
		}
	}
	var none T
	b.Fatalf("no plugin draws a %T", none)
	return none
}

func (c *headless) Deliver(rules ...act.Rule) error { return ceremony.Deliver(c.hosts, rules...) }
