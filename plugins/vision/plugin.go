package vision

import (
	"fmt"
	"github.com/kjkrol/gram/plugin/host"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Plugin wires vision into a Stage over world.Plugin's space.
// It publishes what entities can see; what to do about it is a behavior's business.
type Plugin struct {
	worldPlugin *world.Plugin
	module      *module
	renderer    *Renderer
	style       ConeStyle
	shadow      *Shadow
	groundStep  float64

	sightings host.PairHost[Sighting]
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin builds the vision plugin over worldPlugin's shared spatial index.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	return &Plugin{worldPlugin: worldPlugin}
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.vision" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	var h *heights
	if p.worldPlugin.Quasi3D() {
		h = &heights{groundOf: p.worldPlugin.Ground, step: p.groundStep, bend: p.worldPlugin.Scale().Bend()}
	}
	p.module = newModule(p.worldPlugin.Space(), &p.sightings, h, p.worldPlugin.Cover)
	p.module.clock = p.worldPlugin.Clock()
	ctx.UseModule(p.module)
	return nil
}

// RunPlan hands the scan to the simulation; call it after world's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer builds the cone renderer; atlas is unused, vision draws primitives.
func (p *Plugin) WithRenderer(render.AtlasSource) {
	p.renderer = NewRenderer(p.worldPlugin.Space()).WithGround(p.worldPlugin.Ground)
	if p.style != nil {
		p.renderer.WithStyle(p.style)
	}
	if p.shadow != nil {
		p.renderer.WithShadow(*p.shadow)
	}
}

func (p *Plugin) Renderer() render.Layer {
	if p.renderer == nil {
		return nil
	}
	return p.renderer
}

// EventHandler is a no-op — vision reads no input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable returns nil: vision keeps no state beside its components.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// RegisterBehavior hosts a Between of Sighting, run once per observer; call before Use.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		if err := p.sightings.Add(b); err != nil {
			return fmt.Errorf("%w in %s — it takes Between for Sighting", err, p.Name())
		}
	}
	return nil
}

// =================================================================
// vision-specific
// =================================================================

// WithStyle sets how cones are drawn, in place of DefaultConeStyle; call before Use.
func (p *Plugin) WithStyle(style ConeStyle) *Plugin {
	p.style = style
	return p
}

// WithShadow sets how the ground out of sight is shaded, in place of DefaultShadow; call before Use.
func (p *Plugin) WithShadow(shadow Shadow) *Plugin {
	p.shadow = &shadow
	return p
}

// WithGroundStep sets how far apart a Quasi3D scan samples the ground along a ray, in place of the
// board's cell; a longer step is a cheaper scan. Call before Use.
func (p *Plugin) WithGroundStep(step float64) *Plugin {
	p.groundStep = step
	return p
}
