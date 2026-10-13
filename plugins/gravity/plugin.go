package gravity

import (
	"fmt"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// metres a second squared
const earth = 9.81

// Config is how a world's gravity begins.
type Config struct {
	// Pull is the pull the world begins under; zero, the Earth's as its figures feel it: −Z in a
	// world with heights, +Y (down the screen) in a flat one.
	Pull Pull
}

// Plugin is gravity over the entities of a world that carry a Body: every step it carries each
// by its drift, stands it on what bears it, lets it fall and lands it, a Landing for the rules
// it hosts.
type Plugin struct {
	*world.Self

	worldPlugin *world.Plugin
	surfaces    func() ground.Surfaces
	module      *module

	landings host.Rules[Landing]
	leaps    command.Queue[Leap]
	aways    command.Queue[Away]
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin builds the gravity of w as cfg begins it; a pull across a world with heights, or up
// a flat one, panics.
func NewPlugin(w *world.Plugin, cfg Config) *Plugin {
	pull := cfg.Pull
	g := w.Scale().Figure(earth)
	switch {
	case pull == Pull{} && w.HasHeights():
		pull.Z = -g
	case pull == Pull{}:
		pull.Y = g
	case w.HasHeights() && (pull.X != 0 || pull.Y != 0):
		panic(fmt.Sprintf("gravity: a world with heights pulls along Z, not across: %+v", pull))
	case !w.HasHeights() && pull.Z != 0:
		panic(fmt.Sprintf("gravity: a flat world pulls across, not along Z: %+v", pull))
	}
	p := &Plugin{worldPlugin: w}
	p.Self = world.NewSelf(w, p, comp.Const(pull))
	return p
}

// WithSurfaces gives the plugin what bears a body in a world with heights (the board's
// Surfaces); without it the ground lies at 0. A flat world panics; call before Use.
func (p *Plugin) WithSurfaces(surfaces func() ground.Surfaces) *Plugin {
	if !p.worldPlugin.HasHeights() {
		panic("gravity: a flat world has no surfaces — a body stands on what collision found")
	}
	p.surfaces = surfaces
	return p
}

func (p *Plugin) Name() string { return "gram.gravity" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	ctx.Hosts(&p.landings)
	return nil
}

// RunPlan runs the fall in the simulation; call it right after collision's.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer does nothing: gravity draws nothing.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is nil: gravity draws nothing.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is nil: gravity reads no input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the pull and the drifts are on the entities.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
