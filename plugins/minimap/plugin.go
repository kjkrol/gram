package minimap

import (
	"fmt"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

// Config is how far round its camera a minimap shows and whether it turns with it.
type Config struct {
	// Reach is how far from the camera's middle a minimap shows, to its picture's nearer edge,
	// in world units; never past a world's last band.
	Reach float64
	// Turning has a minimap turn with its camera, the way the camera looks running up it; else
	// north is up.
	Turning bool
}

// Ground is what lies under a minimap: Miniature is the world's ground as a picture, perUnit
// pixels a world unit from its top-left corner, nil while there is none.
type Ground interface {
	Miniature() (img *render.Image, perUnit float32)
}

// Plugin is the minimaps: small pictures round the players' cameras of the ground and the marks
// of the kinds that have one, a click on one taking its camera there.
type Plugin struct {
	*world.Self

	w        *world.Plugin
	ground   Ground
	cfg      Config
	maps     []*minimap
	switches command.Queue[Switch]
	turns    command.Queue[Turn]
	kept     *plugin.Kept[Settings]

	Res Resources
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Tunable = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)

// NewPlugin makes the minimaps of the world w over ground, nil for none, as cfg says; a reach past
// the world's last band panics.
func NewPlugin(w *world.Plugin, ground Ground, cfg Config) *Plugin {
	checkReach(w, cfg.Reach)
	p := &Plugin{w: w, ground: ground, cfg: cfg}
	p.Res.Settings.Turning = cfg.Turning
	p.kept = plugin.Keep(&p.Res.Settings)
	p.Self = world.NewSelf(w, p)
	return p
}

func checkReach(w *world.Plugin, reach float64) {
	bands := w.Res.Config.Sectors.Bands
	if len(bands) > 0 && reach > bands[len(bands)-1].Reach {
		panic(fmt.Sprintf("minimap: a reach of %v past the world's last band, %v", reach,
			bands[len(bands)-1].Reach))
	}
}

func (p *Plugin) Name() string                       { return "gram.minimap" }
func (p *Plugin) Install(plugin.Installer) error     { return nil }
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the minimaps' Settings, kept whole.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Settings loaded and shows or hides the minimaps as they say.
func (p *Plugin) Restore() {
	p.kept.Restore()
	p.Show(!p.Res.Settings.Hidden)
}

// RunPlan carries the minimaps' commands out and has each follow its camera; after the cameras'.
func (p *Plugin) RunPlan(goke.RunCtx, time.Duration) {
	p.switches.Drain(func(i command.Issued[Switch]) { p.Show(i.Command.Shown) })
	p.turns.Drain(func(i command.Issued[Turn]) { p.SetTurning(i.Command.On) })
	for _, m := range p.maps {
		m.follow(p.Res.Settings.Turning)
	}
}

// For is the minimap of cam as a picture for a scene to place, a click on it moving cam there as
// issuer's command (cameras.CenterAt).
func (p *Plugin) For(cam camera.Camera, issuer command.Issuer) *ui.Element {
	space := p.w.Res.Config.Space
	m := &minimap{p: p, of: cam, issuer: issuer, lens: newLens(
		geom.NewVec(float64(space.Width), float64(space.Height)),
		space.Edges.WrapsX(), space.Edges.WrapsY(), p.cfg.Reach)}
	m.follow(p.Res.Settings.Turning)
	m.element = ui.Image(&paced{Feed: render.NewFeed(m.lens, m.picture())}).Input(&clicks{m: m})
	if p.Res.Settings.Hidden {
		m.element.Hidden()
	}
	p.maps = append(p.maps, m)
	return m.element
}

// Show has the minimaps shown, or hidden (Settings.Hidden).
func (p *Plugin) Show(on bool) {
	p.Res.Settings.Hidden = !on
	for _, m := range p.maps {
		if on {
			m.element.Show()
		} else {
			m.element.Hide()
		}
	}
}

// Shown reports whether the minimaps are shown.
func (p *Plugin) Shown() bool { return !p.Res.Settings.Hidden }

// SetTurning has the minimaps turn with their cameras, or keep north up (Settings.Turning).
func (p *Plugin) SetTurning(on bool) { p.Res.Settings.Turning = on }

// Turning reports whether the minimaps turn with their cameras.
func (p *Plugin) Turning() bool { return p.Res.Settings.Turning }

// SetReach has the minimaps show reach round their cameras; past the world's last band panics.
func (p *Plugin) SetReach(reach float64) {
	checkReach(p.w, reach)
	p.cfg.Reach = reach
	for _, m := range p.maps {
		m.lens.reach = reach
		m.lens.fit()
	}
}
