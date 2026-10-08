package selection

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Plugin wires selection into a Game; it depends on world and defines the Select command.
type Plugin struct {
	*world.Self // its own entity: its knobs, the roles it plays, the effects it is under

	worldPlugin  *world.Plugin
	selects      control.Queue[Select]
	marqueeQueue control.Queue[Marquee]
	effectCmds   control.Queue[effectCommand]
	hovers       control.Queue[Hover]
	allows       control.Queue[Allow]
	forbids      control.Queue[Forbid]
	marquees     marquees
	module       *module
	renderer     *Renderer
	tags         Tags
}

var _ plugin.Plugin = (*Plugin)(nil)

// NewPlugin builds the selection plugin over worldPlugin's space; hand it to the players plugin
// for its Select command and default bindings.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	reg := worldPlugin.Kinds()
	tags := Tags{
		Selectable: reg.DefineTag[Family]("selection.selectable"),
		Selected:   reg.DefineTag[Family]("selection.selected"),
		Hovered:    reg.DefineTag[Family]("selection.hovered"),
	}
	worldPlugin.Roster().Unit.Default(comp.Marks[Family]()) // every unit may be told Allow
	return &Plugin{Self: world.NewSelf(worldPlugin, "gram.selection"), worldPlugin: worldPlugin, tags: tags}
}

// Tags returns selection's tags: for the plugins reading who is Selected.
func (p *Plugin) Tags() Tags { return p.tags }

// Chosen is the one Selected unit player by owns (owner.Obeys) — whom FollowKey fastens
// the camera over; false with none, or several. Nothing before the plugin is installed.
func (p *Plugin) Chosen(by control.PlayerID) (uid.UID64, bool) {
	if p.module == nil {
		return 0, false
	}
	return p.module.sys.chosen(by)
}

// IsSelected reports whether an entity carrying marks is selected: a drawing rule's condition —
// vision.NewPlugin(w).WithViews(render.Show(sel.IsSelected)).
func (p *Plugin) IsSelected(marks tag.Tags[Family]) bool { return marks.Has(p.tags.Selected) }

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.selection" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	sys := NewSelectionSystem(&p.selects, p.worldPlugin.Space(), p.tags, p.worldPlugin.Look)
	sys.marqueeQueue, sys.marquees = &p.marqueeQueue, &p.marquees
	sys.effectCmds, sys.effects = &p.effectCmds, p.worldPlugin.Effects()
	sys.hovers = &p.hovers
	sys.allows, sys.forbids = &p.allows, &p.forbids
	p.module = &module{sys: sys}
	ctx.UseModule(p.module)
	return nil
}

func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer builds the renderer of the highlights and of the box being dragged; atlas is
// unused, selection draws primitives.
func (p *Plugin) WithRenderer(atlas render.AtlasSource) {
	p.renderer = NewRenderer(p.tags.Selected, p.worldPlugin.Look)
	p.renderer.marquees = &p.marquees
}

func (p *Plugin) Renderer() render.Layer {
	if p.renderer == nil {
		return nil
	}
	return p.renderer
}

// EventHandler returns nil — a player's bindings (DefaultBindings) issue the Select commands.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is a no-op — selection has nothing to persist.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
