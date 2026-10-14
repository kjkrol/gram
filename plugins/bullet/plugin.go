package bullet

import (
	"errors"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Plugin fires shots and flies them, hosting the rules of their Landing, Resting and Blast.
type Plugin struct {
	*world.Self

	worldPlugin *world.Plugin
	shots       *Shots
	selected    tag.Tag[selection.Family]
	ground      func() ground.Heights // nil: the ground lies at 0
	module      *module

	landings host.Rules[Landing]
	restings host.Rules[Resting]
	blasts   host.PairRules[Blast]

	shoots command.Queue[Shoot]
	bursts command.Queue[Burst]
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin builds the bullet plugin over w, firing a player's Shoot from the units sel marks
// Selected.
func NewPlugin(w *world.Plugin, sel *selection.Plugin) *Plugin {
	p := &Plugin{worldPlugin: w, shots: NewShots(w), selected: sel.Tags().Selected}
	p.Self = world.NewSelf(w, p)
	return p
}

// Shots is the plugin's register of the kinds of shots.
func (p *Plugin) Shots() *Shots { return p.shots }

// WithGround gives the plugin the ground a thrown shot comes down on, read as needed; without it
// the ground lies at 0.
func (p *Plugin) WithGround(heights func() ground.Heights) *Plugin {
	p.ground = heights
	return p
}

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

// RunPlan fires the Shoots given and hands the flights to the simulation. Call it before the
// world's RunPlan, so a step's flights are tested by collision in the same step.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// Renderer is nil: bullet draws nothing of its own.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler is nil: bullet handles no input of its own.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: a shot's state is on its entity.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
