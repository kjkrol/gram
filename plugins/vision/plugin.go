package vision

import (
	"fmt"
	"log"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Plugin wires vision into a Stage over the world's space. It publishes what entities can see;
// what to do about it is a rule's business.
type Plugin struct {
	*world.Self

	worldPlugin *world.Plugin
	module      *module
	renderer    *Renderer
	groundStep  float64
	heights     func() ground.Heights
	cover       func() ground.Cover
	hidden      bool
	workers     int
	frequency   float64
	log         *log.Logger
	cones       command.Queue[Cones]

	sightings host.PairRules[Sighting]
	drawing   render.Rules
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin builds the vision plugin over worldPlugin's space; the views drawn start hidden.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	p := &Plugin{worldPlugin: worldPlugin, hidden: true}
	p.Self = world.NewSelf(worldPlugin, p)
	p.renderer = NewRenderer(worldPlugin.Space()).WithGround(p.groundOf).WithCover(p.coverOf).
		WithScale(worldPlugin.Scale()).WithDrawing(&p.drawing).WithStyles(worldPlugin.Styles)
	p.renderer.Hide(true)
	return p
}

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
	ctx.Draws(p.renderer)
	return nil
}

// RunPlan carries out the commands and runs the scan; call it after the world's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.cones.Drain(func(command.Issued[Cones]) { p.setHidden(!p.hidden) })
	p.module.RunPlan(ctx, d)
}

// EventHandler is nil: the Cones command comes from a player's bindings.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: vision keeps no state beside its components.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// WithViews has only the views of the observers a render.Show rule picks drawn; with none, every
// one is. Call it in the scene, before the first frame.
func (p *Plugin) WithViews(rules ...render.Rule) *Plugin {
	if err := p.drawing.Add(rules...); err != nil {
		panic(fmt.Errorf("%w in %s", err, p.Name()))
	}
	return p
}

// ShowCones shows every view drawn, as the Cones command does.
func (p *Plugin) ShowCones() { p.setHidden(false) }

// HideCones hides every view drawn; the scan goes on.
func (p *Plugin) HideCones() { p.setHidden(true) }

// ConesShown reports whether the views drawn are shown.
func (p *Plugin) ConesShown() bool { return !p.hidden }

func (p *Plugin) setHidden(hidden bool) {
	p.hidden = hidden
	p.renderer.Hide(hidden)
}

// WithStyle sets how every cone is drawn in place of the kinds' Cones: a composer of its own,
// on the CPU.
func (p *Plugin) WithStyle(style ConeStyle) *Plugin {
	p.renderer.WithStyle(style)
	return p
}

// WithShadow sets how the ground out of sight is shaded, in place of DefaultShadow.
func (p *Plugin) WithShadow(shadow Shadow) *Plugin {
	p.renderer.WithShadow(shadow)
	return p
}

// WithBoard has sight follow brd's ground and be held back by its cover; call before Use.
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

func (p *Plugin) groundOf() ground.Heights {
	if p.heights == nil {
		return nil
	}
	return p.heights()
}

func (p *Plugin) coverOf() ground.Cover {
	if p.cover == nil {
		return nil
	}
	return p.cover()
}

// WithGroundStep sets how far apart a scan with heights samples a ground without a
// ground.Profile along a ray, in place of the board's cell; call before Use.
func (p *Plugin) WithGroundStep(step float64) *Plugin {
	p.groundStep = step
	p.renderer.WithGroundStep(step)
	return p
}

// WithLog has vision write a line to l the first time one entity sees another; call before Use.
func (p *Plugin) WithLog(l *log.Logger) *Plugin {
	p.log = l
	return p
}

// WithFrequency has every observer scanned perSecond times a second of game time in place of
// every step (0), its rules going on with what it saw last between scans; call before Use.
func (p *Plugin) WithFrequency(perSecond float64) *Plugin {
	p.frequency = max(perSecond, 0)
	return p
}

// WithWorkers sets how many goroutines at most share a scan: 0 (as it starts) as many as there
// are CPUs, 1 none. Call before Use.
func (p *Plugin) WithWorkers(n int) *Plugin {
	p.workers = n
	return p
}
