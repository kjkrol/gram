package engine

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/world"
)

// initializer is the game.Initializer bound to one Stage's ecsHost.
type initializer struct {
	host  *ecsHost
	world *world.Plugin
	tps   *game.TPS
	// handlers are the plugin.CommandHandlers used before the world, for it to carry
	handlers []plugin.CommandHandler
	// hosts take the rules of the moments the plugins used so far catch, in the order of Use
	hosts []plugin.Host
	// part is the section of the Stage's definition under way, for one built in sections
	part section.Part
}

var (
	_ game.Initializer = (*initializer)(nil)
	_ section.Writer   = (*initializer)(nil)
)

// Section is the part of the Stage being defined; none for a Stage written by hand.
func (c *initializer) Section() section.Part { return c.part }

// Enter says which part of the Stage begins: package game/stage drives it.
func (c *initializer) Enter(p section.Part) { c.part = p }

func (c *initializer) UseModule(m goke.Module) { c.host.useModule(m) }

func (c *initializer) Setup(providers ...goke.SetupProvider) { c.host.setup(providers...) }

func (c *initializer) RegSys(factory func() goke.System) goke.Runnable {
	return c.host.regSys(factory)
}

func (c *initializer) ECS() *goke.ECS { return c.host.ecs }

// Use installs p, rejecting a duplicate Name and any Plugin the engine installs itself.
func (c *initializer) Use(p plugin.Plugin) error {
	if err := section.Check(c, fmt.Sprintf("plugin %q used", p.Name()), section.Plugins); err != nil {
		return err
	}
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
	return p.Install(c)
}

// PluginScenes are the scenes of the plugins used so far that have their own (game.Scenic), in
// the order of Use: for the Stage's stack.
func (c *initializer) PluginScenes() []game.Scene {
	var scenes []game.Scene
	for _, v := range c.host.tracked {
		if s, ok := v.(game.Scenic); ok {
			scenes = append(scenes, s.Scenes()...)
		}
	}
	return scenes
}

// Hosts keeps the hosts of the rules of the moments a plugin catches, in the order of Use.
func (c *initializer) Hosts(hosts ...plugin.Host) { c.hosts = append(c.hosts, hosts...) }

// deliver hands the rules of every role somebody plays — a kind, a cell, a plugin — to the host
// of their moment: what the engine does once Init returns.
func (c *initializer) deliver() error {
	if c.world == nil {
		return nil
	}
	return hosts.Deliver(c.hosts, c.world.Kinds().Played()...)
}

// Track registers s for Save and Load under its Go type name; tracked after a Load, s gets the
// state the save holds for it at once.
func (c *initializer) Track(s plugin.Serializable) error {
	c.host.track(s)
	return c.host.loadLate(s)
}

func (c *initializer) UseWorld(cfg world.Config) *world.Plugin {
	section.Must(c, "the world used", section.Plugins)
	if c.world != nil {
		panic("gram: UseWorld called more than once in the same Stage")
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
