package vision

import (
	"fmt"
	"log"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Plugin wires vision into a Stage over world.Plugin's space.
// It publishes what entities can see; what to do about it is a rule's business.
type Plugin struct {
	*world.Self // its own entity: its knobs, the roles it plays, the effects it is under

	worldPlugin *world.Plugin
	module      *module
	renderer    *Renderer
	style       ConeStyle
	shadow      *Shadow
	groundStep  float64
	heights     func() ground.Heights // the ground sight follows; nil, flat
	cover       func() ground.Cover   // what holds sight back; nil, nothing
	hidden      bool                  // the views drawn are hidden, as they start — see Cones
	workers     int                   // how many goroutines at most share a scan: 0 all the CPUs, 1 none
	frequency   float64               // how often each observer is scanned a second; 0, every step
	log         *log.Logger           // a line the first time one entity sees another, nil for none
	cones       control.Queue[Cones]

	sightings plugin.PairRules[Sighting]
	drawing   render.Rules // which views are drawn; none, every one
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin builds the vision plugin over worldPlugin's shared spatial index; the views drawn
// start hidden — see Cones.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	p := &Plugin{worldPlugin: worldPlugin, hidden: true}
	p.Self = world.NewSelf(worldPlugin, p)
	return p
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
	p.module = newModule(p.worldPlugin.Space(), &p.sightings, h, p.coverOf, p.workers)
	p.module.sys.Every(p.frequency)
	p.module.clock = p.worldPlugin.Clock()
	p.module.sys.tick, p.module.sys.log = p.worldPlugin.Tick, p.log
	ctx.UseModule(p.module)
	ctx.Hosts(&p.sightings)
	return nil
}

// RunPlan carries out the commands, then hands the scan to the simulation; call it after world's
// RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.cones.Drain(func(control.Issued[Cones]) { p.setHidden(!p.hidden) })
	p.module.RunPlan(ctx, d)
}

// WithRenderer builds the cone renderer; atlas is unused, vision draws primitives.
func (p *Plugin) WithRenderer(render.AtlasSource) {
	p.renderer = NewRenderer(p.worldPlugin.Space()).WithGround(p.groundOf).WithGroundStep(p.groundStep).WithCover(p.coverOf).WithScale(p.worldPlugin.Scale()).WithDrawing(&p.drawing)
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

// WithViews has the views drawn as rules say, every frame: render.Show picks the observers
// whose views are drawn — the selected ones, say (render.Show(selected.In)); with none, every
// one is. Drawing is the scene's: call it as the renderer is made (WithRenderer).
func (p *Plugin) WithViews(rules ...render.Rule) *Plugin {
	if err := p.drawing.Add(rules...); err != nil {
		panic(fmt.Errorf("%w in %s", err, p.Name()))
	}
	return p
}

// =================================================================
// vision-specific
// =================================================================

// ShowCones shows every view drawn — the cones and the shadows — as the Cones command does.
func (p *Plugin) ShowCones() { p.setHidden(false) }

// HideCones hides every view drawn, as they start; the scan goes on.
func (p *Plugin) HideCones() { p.setHidden(true) }

// ConesShown reports whether the views drawn are shown.
func (p *Plugin) ConesShown() bool { return !p.hidden }

func (p *Plugin) setHidden(hidden bool) {
	p.hidden = hidden
	if p.renderer != nil {
		p.renderer.Hide(hidden)
	}
}

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
func (p *Plugin) WithHeights(heights func() ground.Heights) *Plugin {
	p.heights = heights
	return p
}

// WithCover has sight held back by the cover cover gives when a scan starts; nil, nothing.
func (p *Plugin) WithCover(cover func() ground.Cover) *Plugin {
	p.cover = cover
	return p
}

// groundOf is the ground as it stands now, nil without one.
func (p *Plugin) groundOf() ground.Heights {
	if p.heights == nil {
		return nil
	}
	return p.heights()
}

// coverOf is the cover as it stands now, nil without one.
func (p *Plugin) coverOf() ground.Cover {
	if p.cover == nil {
		return nil
	}
	return p.cover()
}

// WithGroundStep sets how far apart a scan with heights samples the ground along a ray, in place of the
// board's cell, where the ground has no ground.Profile — a topography's relief has, and is read
// exactly; a longer step is a cheaper scan. Call before Use.
func (p *Plugin) WithGroundStep(step float64) *Plugin {
	p.groundStep = step
	return p
}

// WithLog has the vision write a line to l the first time one entity sees another; call before
// Use.
func (p *Plugin) WithLog(l *log.Logger) *Plugin {
	p.log = l
	return p
}

// WithFrequency has every observer scanned perSecond times a second of game time, the observers
// spread evenly over the steps, in place of every step (0, as it starts); between its scans an
// observer's rules of a Sighting go on with what it saw last. A lower frequency is a cheaper
// scan. Call before Use.
func (p *Plugin) WithFrequency(perSecond float64) *Plugin {
	p.frequency = max(perSecond, 0)
	return p
}

// WithWorkers sets how many goroutines at most share a scan's observers: 0, as it starts, as many
// as there are CPUs; 1 none. Call before Use.
func (p *Plugin) WithWorkers(n int) *Plugin {
	p.workers = n
	return p
}
