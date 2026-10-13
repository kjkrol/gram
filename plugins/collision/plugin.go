package collision

import (
	"log"
	"time"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Plugin wires collision into a Stage over the world's space.
type Plugin struct {
	*world.Self

	worldPlugin *world.Plugin
	module      *module

	pairs    host.PairRules[Meeting]
	entities host.Rules[Struck]
	field    Field
	stats    *ContactStats
	log      *log.Logger
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin builds the collision plugin over worldPlugin's space.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	worldPlugin.Roster().Unit.Default(comp.Const(Collider{}))
	worldPlugin.Roster().Unit.Default(comp.Const(Physics{}))
	p := &Plugin{worldPlugin: worldPlugin}
	p.Self = world.NewSelf(worldPlugin, p)
	return p
}

func (p *Plugin) Name() string { return "gram.collision" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = newModule(p.worldPlugin.Space(), ctx.ECS(), &p.pairs, &p.entities)
	p.module.fieldOf, p.module.clock = func() Field { return p.field }, p.worldPlugin.Clock()
	p.module.heights = p.worldPlugin.HasHeights()
	p.module.stats, p.module.log = p.stats, p.log
	p.module.tick = p.worldPlugin.Tick
	ctx.UseModule(p.module)
	ctx.Hosts(&p.pairs, &p.entities)
	return nil
}

// WithStats has collision count every contact into s; call before Use.
func (p *Plugin) WithStats(s *ContactStats) *Plugin {
	p.stats = s
	return p
}

// WithLog has collision write a line to l for every contact between entities; call before Use.
func (p *Plugin) WithLog(l *log.Logger) *Plugin {
	p.log = l
	return p
}

// WithField makes f the solid ground colliders are pushed out of; call before Use.
func (p *Plugin) WithField(f Field) *Plugin {
	p.field = f
	return p
}

// RunPlan runs the collisions in the simulation; call it after the world's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer does nothing: collision draws nothing.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is nil: collision draws nothing.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is nil: collision reads no input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: collision keeps no state beside its components.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
