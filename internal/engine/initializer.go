package engine

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
)

// initializer is the game.Initializer bound to one Stage's ecsHost.
type initializer struct {
	host  *ecsHost
	world *world.Plugin
	tps   *game.TPS
	// handlers are the plugin.CommandHandlers used before the world, for it to carry
	handlers []plugin.CommandHandler
	// used are the plugins installed so far, in the order they were: the hosts Hook tries
	used []any

	screenWidth, screenHeight int
}

var _ game.Initializer = (*initializer)(nil)

func (c *initializer) UseModule(m goke.Module) { c.host.useModule(m) }

func (c *initializer) Setup(providers ...goke.SetupProvider) { c.host.setup(providers...) }

func (c *initializer) RegSys(factory func() goke.System) goke.Runnable {
	return c.host.regSys(factory)
}

func (c *initializer) ECS() *goke.ECS { return c.host.ecs }

// Use installs p, rejecting a duplicate Name and any Plugin the engine installs itself.
func (c *initializer) Use(p plugin.Plugin) error {
	if _, ok := p.(plugin.Builtin); ok {
		return fmt.Errorf("gram: %q is installed by the engine itself — do not Use it yourself", p.Name())
	}
	return c.use(p)
}

// useBuiltin installs an engine-managed Plugin.
func (c *initializer) useBuiltin(p plugin.Plugin) error { return c.use(p) }

// use checks p's Name is new, registers its Serializable, tracks it and runs its Install; the
// world carries the commands of a plugin.CommandHandler its entities give themselves.
func (c *initializer) use(p plugin.Plugin) error {
	if c.host.names == nil {
		c.host.names = make(map[string]bool)
	}
	if c.host.names[p.Name()] {
		return fmt.Errorf("gram: plugin %q already used", p.Name())
	}
	c.host.names[p.Name()] = true
	if s := p.Serializable(); s != nil {
		c.host.resources.register(p.Name(), s)
	}
	c.host.track(p)
	if h, ok := p.(plugin.CommandHandler); ok && p != plugin.Plugin(c.world) {
		if c.world == nil {
			c.handlers = append(c.handlers, h)
		} else if err := c.world.Carry(h); err != nil {
			return fmt.Errorf("gram: %q: %w", p.Name(), err)
		}
	}
	if err := p.Install(c); err != nil {
		return err
	}
	c.used = append(c.used, p)
	return nil
}

// Hook hooks each rule on the first plugin used that hosts its moment.
func (c *initializer) Hook(rules ...rule.Rule) error { return rule.HookOn(c.used, rules...) }

// Track registers s for Save and Load under its Go type name.
func (c *initializer) Track(s plugin.Serializable) error {
	c.host.track(s)
	return nil
}

func (c *initializer) UseWorld(cfg world.Config) *world.Plugin {
	if c.world != nil {
		panic("gram: UseWorld called more than once in the same Stage")
	}
	if cfg.Camera.ViewportWidth == 0 && cfg.Camera.ViewportHeight == 0 {
		cfg.Camera.ViewportWidth = uint32(c.screenWidth)
		cfg.Camera.ViewportHeight = uint32(c.screenHeight)
	}
	c.world = world.NewPlugin(cfg)
	if err := c.useBuiltin(c.world); err != nil {
		panic(err)
	}
	if err := c.world.Carry(c.handlers...); err != nil {
		panic(err)
	}
	return c.world
}

func (c *initializer) TPS() *game.TPS { return c.tps }
