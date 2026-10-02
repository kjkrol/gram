package selection

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
)

// Plugin wires selection into a Game; it depends on world and defines the Select command.
type Plugin struct {
	worldPlugin  *world.Plugin
	selects      control.Queue[Select]
	marqueeQueue control.Queue[Marquee]
	follows      control.Queue[Follow]
	applies      control.Queue[Apply]
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
		Followed:   reg.DefineTag[Family]("selection.followed"),
	}
	return &Plugin{worldPlugin: worldPlugin, tags: tags}
}

// Tags returns selection's tags, to give Selectable to a kind or to read Selected.
func (p *Plugin) Tags() Tags { return p.tags }

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.selection" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	sys := NewSelectionSystem(&p.selects, p.worldPlugin.Space(), p.tags, p.worldPlugin.Look)
	sys.marqueeQueue, sys.marquees = &p.marqueeQueue, &p.marquees
	sys.applies = &p.applies
	p.module = &module{sys: sys, follow: NewFollowSystem(&p.follows, p.tags)}
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

// Hook reports ErrUnhosted — selection hosts no rules.
func (p *Plugin) Hook(rules ...rule.Rule) error {
	for _, b := range rules {
		return fmt.Errorf("%w: %T in %s", rule.ErrUnhosted, b, p.Name())
	}
	return nil
}
