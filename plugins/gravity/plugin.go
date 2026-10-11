package gravity

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// earth is the pull at the Earth's surface, in metres a second squared.
const earth = 9.81

// Config is how a world's gravity begins.
type Config struct {
	// Pull is the pull the world begins under; zero, the Earth's as its figures feel it
	// (world.Scale.Figure): down, −Z, in a world with heights, down the screen, +Y, in a flat one.
	Pull Pull
}

// Plugin is gravity over the entities of a world that carry a Body: every step of the
// simulation it carries each by its drift, stands it on what bears it, lets it fall where nothing
// does — unit.Airborne on while it flies — and lands it, a Landing for the rules it hosts. A Leap
// throws a body up, an Away off. A world with heights pulls along −Z, a body standing on the
// ground or a deck within its step (ground.Surfaces); a flat one across, a body standing on
// whatever collision found it touching against the pull. Depends on world and, for the bodies'
// markers, board.
type Plugin struct {
	*world.Self // its own entity: the Pull, the roles it plays, the effects it is under

	worldPlugin *world.Plugin
	surfaces    func() ground.Surfaces // nil: the ground lies at 0
	module      *module

	landings plugin.Rules[Landing]
	leaps    control.Queue[Leap]
	aways    control.Queue[Away]
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

// WithSurfaces gives the plugin what bears a body in a world with heights, read as needed — the
// board's (board.Plugin.Surfaces, nil before Setup); without it the ground lies at 0. A flat world
// panics. Call before Use.
func (p *Plugin) WithSurfaces(surfaces func() ground.Surfaces) *Plugin {
	if !p.worldPlugin.HasHeights() {
		panic("gravity: a flat world has no surfaces — a body stands on what collision found")
	}
	p.surfaces = surfaces
	return p
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.gravity" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	ctx.Hosts(&p.landings)
	return nil
}

// RunPlan hands the fall to the simulation; call it right after collision's.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op — gravity draws nothing.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is a no-op — gravity has no render.Renderer of its own.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is a no-op — gravity has no control.EventHandler of its own.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is a no-op — the pull and the drifts are on the entities.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
