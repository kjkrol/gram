package selection

import (
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
)

// Plugin wires selection into a Game; it depends on world and defines the Select command.
type Plugin struct {
	*world.Self

	worldPlugin  *world.Plugin
	selects      command.Queue[Select]
	marqueeQueue command.Queue[Marquee]
	effectCmds   command.Queue[effectCommand]
	hovers       command.Queue[Hover]
	allows       command.Queue[Allow]
	forbids      command.Queue[Forbid]
	marquees     marquees
	module       *module
	renderer     *Renderer
	tags         Tags
	read         Read
	reads        parallel.Reads[*Read]
	forks        []*Read
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.KindTemplater = (*Plugin)(nil)

// NewPlugin builds the selection plugin over worldPlugin's space; hand it to the players plugin
// for its Select command and default bindings.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	reg := worldPlugin.Kinds()
	tags := Tags{
		Selectable: reg.DefineTag[Family]("selection.selectable"),
		Selected:   reg.DefineTag[Family]("selection.selected"),
		Hovered:    reg.DefineTag[Family]("selection.hovered"),
	}
	p := &Plugin{worldPlugin: worldPlugin, tags: tags}
	p.read.p = p
	p.reads = parallel.Reads[*Read]{&p.read}
	p.Self = world.NewSelf(worldPlugin, p)
	p.renderer = NewRenderer(tags.Selected, worldPlugin.Read(0).Look, worldPlugin.Read(0).Styles)
	p.renderer.marquees = &p.marquees
	return p
}

// Tags returns selection's tags: for the plugins reading who is Selected.
func (p *Plugin) Tags() Tags { return p.tags }

// KindTemplates is what selection asks of the kinds a game defines: every unit marked, so it may
// be told Allow.
func (p *Plugin) KindTemplates(r *kind.Roster) {
	r.Default(motion.UnitTemplate, comp.Marks[Family]())
}

func (p *Plugin) Name() string { return "gram.selection" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	sys := NewSelectionSystem(&p.selects, p.worldPlugin.Write().Space(), p.tags,
		p.worldPlugin.Read(0).Look)
	sys.marqueeQueue, sys.marquees = &p.marqueeQueue, &p.marquees
	sys.effectCmds, sys.effects = &p.effectCmds, p.worldPlugin.Write().Effects()
	sys.hovers = &p.hovers
	sys.allows, sys.forbids = &p.allows, &p.forbids
	p.module = &module{sys: sys, ready: p.readied}
	ctx.UseModule(p.module)
	ctx.Draws(p.renderer)
	p.reads = parallel.NewReads(&p.read, ctx.Pool())
	return nil
}

func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// EventHandler is nil: a player's bindings issue the commands.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: selection has nothing to persist.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
