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
	"github.com/kjkrol/gram/plugins/world/sector"
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
	covers      func(w int) ground.Cover // worker w's
	workers     int
	frequency   float64
	log         *log.Logger
	cones       command.Queue[Cones]

	sightings host.PairRules[Sighting]
	drawing   render.Rules
	kept      *plugin.Kept[Settings]

	Res   Resources
	read  Read
	write Write
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Tunable = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)

// NewPlugin builds the vision plugin over worldPlugin's space; the views drawn start hidden.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	p := &Plugin{worldPlugin: worldPlugin}
	p.read.p, p.write.p = p, p
	p.kept = plugin.Keep(&p.Res.Settings)
	p.Self = world.NewSelf(worldPlugin, p)
	p.renderer = NewRenderer(worldPlugin.Read(0).Space()).WithGround(p.groundOf).WithCover(p.coverOf).
		WithScale(worldPlugin.Read(0).Scale()).WithDrawing(&p.drawing).
		WithStyles(worldPlugin.Read(0).Styles).WithSeen(worldPlugin.Read(0).ViewFor)
	p.renderer.Hide(true)
	return p
}

func (p *Plugin) Name() string { return "gram.vision" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	var h *heights
	if p.worldPlugin.Read(0).HasHeights() {
		h = &heights{groundOf: p.groundOf, step: p.groundStep, bend: p.worldPlugin.Read(0).Scale().Bend()}
	}
	p.module = newModule(p.worldPlugin.Read(0).Space(), &p.sightings, h, p.coverOn, p.workers)
	p.module.sys.Pool(ctx.Pool())
	p.module.sys.Paced(p.worldPlugin.Pace(sector.Near))
	p.module.sys.Every(p.frequency)
	p.module.clock = p.worldPlugin.Write().Clock()
	p.module.sys.tick, p.module.sys.log = p.worldPlugin.Read(0).Tick, p.log
	ctx.UseModule(p.module)
	ctx.Hosts(&p.sightings)
	ctx.Draws(p.renderer)
	return nil
}

// RunPlan carries out the commands and runs the scan; call it after the world's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.cones.Drain(func(i command.Issued[Cones]) { p.setHidden(!i.Command.Shown) })
	p.module.RunPlan(ctx, d)
}

// EventHandler is nil: the Cones command comes from a player's bindings.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is vision's Settings, kept whole; its components go with the entities.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Settings loaded and shows the views as they say.
func (p *Plugin) Restore() {
	p.kept.Restore()
	p.setHidden(!p.Res.Settings.Cones)
}

// WithViews has only the views of the observers a render.Show rule picks drawn; with none, every
// one is. Call it in the scene, before the first frame.
func (p *Plugin) WithViews(rules ...render.Rule) *Plugin {
	if err := p.drawing.Add(rules...); err != nil {
		panic(fmt.Errorf("%w in %s", err, p.Name()))
	}
	return p
}

func (p *Plugin) setHidden(hidden bool) {
	p.Res.Settings.Cones = !hidden
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
	p.covers = func(w int) ground.Cover { return brd.Read(w).Cover() }
	return p.WithHeights(brd.Read(0).Heights)
}

// WithHeights has sight follow the ground heights gives when a scan starts; nil, flat.
func (p *Plugin) WithHeights(heights func() ground.Heights) *Plugin {
	p.heights = heights
	return p
}

// WithCover has sight held back by the cover cover gives when a scan starts, on every goroutine
// the scan is shared among; nil, nothing.
func (p *Plugin) WithCover(cover func() ground.Cover) *Plugin {
	p.covers = nil
	if cover != nil {
		p.covers = func(int) ground.Cover { return cover() }
	}
	return p
}

func (p *Plugin) groundOf() ground.Heights {
	if p.heights == nil {
		return nil
	}
	return p.heights()
}

func (p *Plugin) coverOf() ground.Cover { return p.coverOn(0) }

// the cover worker w of the game's pool reads
func (p *Plugin) coverOn(w int) ground.Cover {
	if p.covers == nil {
		return nil
	}
	return p.covers(w)
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

// WithWorkers sets how many goroutines of the game's pool at most share a scan: 0 (as it starts)
// all, 1 none. Call before Use.
func (p *Plugin) WithWorkers(n int) *Plugin {
	p.workers = n
	return p
}
