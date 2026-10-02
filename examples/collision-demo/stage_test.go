package main

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
)

// benchInit is a game.Initializer that drives the real Stage without a window;
// Scene.Layers() is left out.
type benchInit struct {
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

func (c *benchInit) UseWorld(cfg world.Config) *world.Plugin {
	cfg.Camera.ViewportWidth = ScreenWidth
	cfg.Camera.ViewportHeight = ScreenHeight
	c.world = world.NewPlugin(cfg)
	c.tracked = append(c.tracked, c.world)
	if err := c.world.Install(c); err != nil {
		panic(err)
	}
	return c.world
}

// buildStage runs the fresh-spawn half of entering a Stage: Init, Spawn, Populate, Setup.
func buildStage(tb testing.TB) (*goke.ECS, *mainStage) {
	tb.Helper()

	rng = rand.New(rand.NewPCG(0x5eed, 0xc0ffee))

	stage := &mainStage{}
	ctx := &benchInit{ecs: goke.New()}
	if err := stage.Init(ctx); err != nil {
		tb.Fatalf("Init: %v", err)
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
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { stage.Update(rc, d); stage.world.Clock().Replay(rc, d) })

	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)

	return ctx.ecs, stage
}

// benchStep is one tick at the demo's TPS.
const benchStep = time.Second / TPS
