package bullet

import (
	"errors"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Plugin fires shots and flies them: a Shoot spawns a shot of its Ammo — a kind of the world —
// at a unit's muzzle, every step of the simulation flies each shot its Body's speed along its
// way, past the world's step cap and swept by collision, or in an arc, and ends the flight where
// collision found a contact, at its Range, on the ground or at an edge: a Landing for the rules
// it hosts, the shot gone unless its Body Lands. A landed shot lies where it ended — a Resting
// every step — until a Burst, a Blast for every entity within its radius. Depends on world,
// collision (a shot is a sensor of its) and selection (whom a player's Shoot fires from).
type Plugin struct {
	*world.Self // its own entity: its knobs, the roles it plays, the effects it is under

	worldPlugin *world.Plugin
	selected    tag.Tag[selection.Family]
	ground      func() ground.Heights // nil: the ground lies at 0
	module      *module

	landings plugin.Rules[Landing]
	restings plugin.Rules[Resting]
	blasts   plugin.PairRules[Blast]

	shoots control.Queue[Shoot]
	bursts control.Queue[Burst]
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin builds the bullet plugin over w, firing a player's Shoot from the units sel marks
// Selected; make it after collision, which the shots are sensors of.
func NewPlugin(w *world.Plugin, sel *selection.Plugin) *Plugin {
	return &Plugin{Self: world.NewSelf(w, "gram.bullet"), worldPlugin: w, selected: sel.Tags().Selected}
}

// WithGround gives the plugin the ground a thrown shot comes down on, read as needed — the board's
// heights (board.Plugin.Heights, nil before Setup); without it the ground lies at 0. Call before
// Use.
func (p *Plugin) WithGround(heights func() ground.Heights) *Plugin {
	p.ground = heights
	return p
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.bullet" }

// Install refuses a wrapping world: a shot's step must not cross a seam.
func (p *Plugin) Install(ctx plugin.Installer) error {
	if _, _, edges := p.worldPlugin.Space().Bounds(); edges.WrapsX() || edges.WrapsY() {
		return errors.New("bullet: a wrapping world is refused — a shot's step must not cross a seam")
	}
	p.module = newModule(p, ctx.ECS())
	ctx.UseModule(p.module)
	ctx.Hosts(&p.landings, &p.restings, &p.blasts)
	return nil
}

// RunPlan fires the Shoots given and hands the flights to the simulation; call it before world's
// RunPlan, so a step's flights are tested by collision in the same step.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op — the world draws the shots from their kinds.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is a no-op — bullet has no render.Renderer of its own.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is a no-op — bullet has no control.EventHandler of its own.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is a no-op — a shot's state is on its entity.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
