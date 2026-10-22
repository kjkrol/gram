package motion

import (
	"time"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
)

// Plugin moves the world's entities: it steers them, moves each by its velocity and deals with
// those gone past an open edge.
type Plugin struct {
	*world.Self

	module  *module
	movers  host.Rules[Moving]
	leavers host.Rules[Leaving]
	write   Write
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.KindTemplater = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin builds motion over the entities of the world w.
func NewPlugin(w *world.Plugin) *Plugin {
	p := &Plugin{}
	p.write.p = p
	p.module = newModule(w, &p.movers, &p.leavers)
	p.Self = world.NewSelf(w, p)
	return p
}

// KindTemplates is what motion asks of the kinds a game defines: every unit a course its
// steering keeps.
func (p *Plugin) KindTemplates(r *kind.Roster) {
	r.Default(UnitTemplate, comp.Const(steering.Course{}))
}

func (p *Plugin) Name() string { return "gram.motion" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	ctx.UseModule(p.module)
	ctx.Hosts(&p.movers, &p.leavers)
	return nil
}

// RunPlan steers and moves the entities and deals with the leavers; call it right after the
// world's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// EventHandler is nil: motion reads no input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: motion keeps no state beside its components.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
