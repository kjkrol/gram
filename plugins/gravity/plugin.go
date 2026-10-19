package gravity

import (
	"fmt"
	"github.com/kjkrol/gram/plugins/motion"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
)

// metres a second squared
const earth = 9.81

// Config is how a world's gravity begins.
type Config struct {
	// Pull is the pull the world begins under, along Z or across X and Y; zero, the Earth's as its
	// figures feel it: −Z in a world with heights, +Y (down the screen) in a flat one.
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
	across      bool // the pull across X and Y, a side view's; else along Z

	landings host.Rules[Landing]
	leaps    command.Queue[Leap]
	aways    command.Queue[Away]
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin builds the gravity of w as cfg begins it, m carrying what it drifts: along Z, a body
// falling to the ground, or across X and Y, a side view's. A pull both ways, or along Z in a flat
// world, panics.
func NewPlugin(w *world.Plugin, m *motion.Plugin, cfg Config) *Plugin {
	if m == nil {
		panic("gravity: a body drifts by motion, and none was given")
	}
	pull := cfg.Pull
	g := w.Scale().Figure(earth)
	switch {
	case pull == Pull{} && w.HasHeights():
		pull.Z = -g
	case pull == Pull{}:
		pull.Y = g
	case pull.Z != 0 && (pull.X != 0 || pull.Y != 0):
		panic(fmt.Sprintf("gravity: a pull along Z or across, not both: %+v", pull))
	case !w.HasHeights() && pull.Z != 0:
		panic(fmt.Sprintf("gravity: a flat world pulls across, not along Z: %+v", pull))
	}
	p := &Plugin{worldPlugin: w, across: pull.Z == 0}
	p.Self = world.NewSelf(w, p, comp.Const(pull))
	return p
}

// WithSurfaces gives the plugin what bears a body falling along Z (the board's Footing); without
// it the ground lies at 0. A pull across panics; call before Use.
func (p *Plugin) WithSurfaces(surfaces func() ground.Surfaces) *Plugin {
	if p.across {
		panic("gravity: a pull across has no surfaces — a body stands on what collision found")
	}
	p.surfaces = surfaces
	return p
}

func (p *Plugin) Name() string { return "gram.gravity" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p}
	p.module.step = p.module.simulate
	ctx.UseModule(p.module)
	ctx.Hosts(&p.landings)
	return nil
}

// RunPlan runs the fall in the simulation; call it right after collision's.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// EventHandler is nil: gravity reads no input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the pull and the drifts are on the entities.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
