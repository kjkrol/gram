package engine

import (
	"fmt"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type initializer struct {
	host     *ecsHost
	world    *world.Plugin
	tps      *game.TPS
	handlers []plugin.CommandHandler // used before the world, or by a Stage without one
	hosts    []host.Host
	part     section.Part
	draws    []render.Source
	drawn    bool
	looks    []func()
}

var (
	_ game.Initializer = (*initializer)(nil)
	_ section.Writer   = (*initializer)(nil)
)

func (c *initializer) Section() section.Part { return c.part }

func (c *initializer) Enter(p section.Part) { c.part = p }

func (c *initializer) UseModule(m goke.Module) { c.host.useModule(m) }

func (c *initializer) Setup(providers ...goke.SetupProvider) { c.host.setup(providers...) }

func (c *initializer) RegSys(factory func() goke.System) goke.Runnable {
	return c.host.regSys(factory)
}

func (c *initializer) ECS() *goke.ECS { return c.host.ecs }

func (c *initializer) Use(p plugin.Plugin) error {
	if err := section.Check(c, fmt.Sprintf("plugin %q used", p.Name()), section.Plugins); err != nil {
		return err
	}
	if _, ok := p.(plugin.Builtin); ok {
		return fmt.Errorf("gram: %q is installed by the engine itself — do not Use it yourself", p.Name())
	}
	return c.use(p)
}

func (c *initializer) useBuiltin(p plugin.Plugin) error { return c.use(p) }

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

func (c *initializer) PluginScenes() []game.Scene {
	var scenes []game.Scene
	for _, v := range c.host.tracked {
		if s, ok := v.(game.Scenic); ok {
			scenes = append(scenes, s.Scenes()...)
		}
	}
	return scenes
}

func (c *initializer) Hosts(hosts ...host.Host) { c.hosts = append(c.hosts, hosts...) }

func (c *initializer) Draws(sources ...render.Source) {
	if c.drawn {
		panic("gram: a plugin draws after the Stage's picture was made")
	}
	for _, s := range sources {
		if s != nil {
			c.draws = append(c.draws, s)
		}
	}
}

// Drawn is what the Stage's plugins draw, in the order they said it; the picture made of it is
// whole, so a later Draws panics.
func (c *initializer) Drawn() []render.Source {
	c.drawn = true
	return c.draws
}

func (c *initializer) Looks(declare func()) { c.looks = append(c.looks, declare) }

func (c *initializer) look() {
	for _, declare := range c.looks {
		declare()
	}
}

func (c *initializer) deliver() error {
	if c.world == nil {
		return nil
	}
	return ceremony.Deliver(c.hosts, c.world.Kinds().Played()...)
}

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

func (c *initializer) carrier() (*command.Carrier, error) {
	if c.world != nil {
		return c.world.Carrier(), nil
	}
	own := &command.Carrier{}
	for _, h := range c.handlers {
		if err := own.Carry(h.Queues()...); err != nil {
			return nil, err
		}
	}
	return own, nil
}
