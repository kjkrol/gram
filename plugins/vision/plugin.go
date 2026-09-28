package vision

import (
	"fmt"
	"github.com/kjkrol/gram/plugin/host"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
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
	heights     func() board.Heights // the ground sight follows; nil, flat
	cover       func() board.Cover   // what holds sight back; nil, nothing
	hidden      bool                 // the views drawn are hidden, as they start — see Cones
	cones       control.Queue[Cones]

	sightings host.PairHost[Sighting]
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin builds the vision plugin over worldPlugin's shared spatial index; the views drawn
// start hidden — see Cones.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	return &Plugin{worldPlugin: worldPlugin, hidden: true}
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.vision" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	var h *heights
	if p.worldPlugin.HasHeights() {
		h = &heights{groundOf: p.groundOf, step: p.groundStep, bend: p.worldPlugin.Scale().Bend()}
	}
	p.module = newModule(p.worldPlugin.Space(), &p.sightings, h, p.coverOf)
	p.module.clock = p.worldPlugin.Clock()
	ctx.UseModule(p.module)
	return nil
}

// RunPlan carries out the commands, then hands the scan to the simulation; call it after world's
// RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.cones.Drain(func(control.Issued[Cones]) { p.Hide(!p.hidden) })
	p.module.RunPlan(ctx, d)
}

// WithRenderer builds the cone renderer; atlas is unused, vision draws primitives.
func (p *Plugin) WithRenderer(render.AtlasSource) {
	p.renderer = NewRenderer(p.worldPlugin.Space()).WithGround(p.groundOf).WithGroundStep(p.groundStep)
	if p.style != nil {
		p.renderer.WithStyle(p.style)
	}
	if p.shadow != nil {
		p.renderer.WithShadow(*p.shadow)
	}
	p.renderer.Hide(p.hidden)
}

func (p *Plugin) Renderer() render.Layer {
	if p.renderer == nil {
		return nil
	}
	return p.renderer
}

// EventHandler returns nil — a player's bindings (DefaultBindings) issue the Cones commands.
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

// Hide hides every view drawn, or shows them again — what the Cones command toggles.
func (p *Plugin) Hide(hidden bool) {
	p.hidden = hidden
	if p.renderer != nil {
		p.renderer.Hide(hidden)
	}
}

// Hidden reports whether the views drawn are hidden.
func (p *Plugin) Hidden() bool { return p.hidden }

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

// WithBoard has sight follow brd's ground and be held back by what stands on it — walls,
// forests — as the board says (board.Plugin.Heights, Cover); call before Use.
func (p *Plugin) WithBoard(brd *board.Plugin) *Plugin {
	return p.WithHeights(brd.Heights).WithCover(brd.Cover)
}

// WithHeights has sight follow the ground heights gives when a scan starts; nil, flat.
func (p *Plugin) WithHeights(heights func() board.Heights) *Plugin {
	p.heights = heights
	return p
}

// WithCover has sight held back by the cover cover gives when a scan starts; nil, nothing.
func (p *Plugin) WithCover(cover func() board.Cover) *Plugin {
	p.cover = cover
	return p
}

// groundOf is the ground as it stands now, nil without one.
func (p *Plugin) groundOf() board.Heights {
	if p.heights == nil {
		return nil
	}
	return p.heights()
}

// coverOf is the cover as it stands now, nil without one.
func (p *Plugin) coverOf() board.Cover {
	if p.cover == nil {
		return nil
	}
	return p.cover()
}

// WithGroundStep sets how far apart a scan with heights samples the ground along a ray, in place of the
// board's cell; a longer step is a cheaper scan. Call before Use.
func (p *Plugin) WithGroundStep(step float64) *Plugin {
	p.groundStep = step
	return p
}
