package collision

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/render"
)

// Plugin wires the collision engine into a Game — optional, borrows world.Plugin's own Space.
// Must never import collision/hooks; a game registers those with Hook.
type Plugin struct {
	worldPlugin *world.Plugin
	module      *module

	pairs    host.PairHost[Meeting]
	entities host.EachHost[Struck]
	field    Field // the solid ground, nil for none
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin builds the collision plugin over worldPlugin's shared spatial index.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	worldPlugin.Roster().Unit.Default(comp.Const(Collider{}))
	worldPlugin.Roster().Unit.Default(comp.Const(Physics{}))
	return &Plugin{worldPlugin: worldPlugin}
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.collision" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = newModule(p.worldPlugin.Space(), ctx.ECS(), &p.pairs, &p.entities)
	p.module.fieldOf, p.module.clock = func() Field { return p.field }, p.worldPlugin.Clock()
	p.module.tick = p.worldPlugin.Tick
	ctx.UseModule(p.module)
	return nil
}

// WithField makes f the solid ground the engine pushes colliders out of — the board's Solid
// cells (board.Plugin.WithCollision); call before Use.
func (p *Plugin) WithField(f Field) *Plugin {
	p.field = f
	return p
}

// RunPlan hands the collision engine to the simulation; call it after world's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op — collision has no render.Renderer of its own.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is a no-op — collision has no render.Renderer of its own.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is a no-op — collision has no control.EventHandler of its own.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is a no-op — collision has nothing to persist.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// Hook hosts rules (rule.On) of Meeting, a pair, or of Struck; call before Use.
func (p *Plugin) Hook(rules ...plugin.Rule) error {
	return hostAll(&p.pairs, &p.entities, rules)
}
