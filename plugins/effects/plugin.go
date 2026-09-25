package effects

import (
	"fmt"
	"github.com/kjkrol/gram/plugin/host"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Plugin puts effects on entities: temporary changes to their components — tags granted for a
// while, values altered and restored — cast from anywhere. It depends on world.
type Plugin struct {
	worldPlugin *world.Plugin
	defs        []def
	originals   *originals
	idlers      host.EachHost[Idling]
	system      *effectSystem
	module      *module
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Serializable = (*Plugin)(nil)

// NewPlugin builds the effects plugin over worldPlugin.
func NewPlugin(worldPlugin *world.Plugin) *Plugin {
	p := &Plugin{worldPlugin: worldPlugin, originals: newOriginals()}
	p.system = newEffectSystem(worldPlugin, &p.defs, p.originals, &p.idlers)
	return p
}

// Define registers an effect under name; call it in Init, before Use.
func (p *Plugin) Define(name string, spec Spec) ID {
	if p.system.built {
		panic(fmt.Sprintf("effects: %q defined after Install", name))
	}
	if len(p.defs) == 1<<8 {
		panic("effects: at most 256 effects")
	}
	d := def{name: name}
	for _, t := range spec {
		t.apply(&d)
	}
	p.defs = append(p.defs, d)
	return ID(len(p.defs) - 1)
}

// Cast puts effect on id for as long as its Spec says; cast again, it is refreshed unless it
// stacks. The change lands with the plugin's next pass.
func (p *Plugin) Cast(cb *goke.CmdBuf, id uid.UID64, effect ID) {
	p.CastFor(cb, id, effect, p.lasts(effect))
}

// CastFor is Cast for a set time; Forever for one that lasts until Dispel.
func (p *Plugin) CastFor(cb *goke.CmdBuf, id uid.UID64, effect ID, d time.Duration) {
	p.system.cast(cb, id, effect, d)
}

// Dispel ends effect on id with the plugin's next pass; nothing happens without it.
func (p *Plugin) Dispel(id uid.UID64, effect ID) { p.system.dispel(id, effect) }

// Has reports whether id is under effect.
func (p *Plugin) Has(id uid.UID64, effect ID) bool { return p.system.has(id, effect) }

func (p *Plugin) lasts(effect ID) time.Duration {
	if d := p.defs[effect].lasts; d > 0 {
		return d
	}
	return Forever
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.effects" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{system: p.system}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan advances every effect; call it after world's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op — effects draw nothing of their own.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer returns nil — effects draw nothing of their own.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler returns nil — effects take no input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable returns the plugin: the originals it holds for altered components.
func (p *Plugin) Serializable() plugin.Serializable { return p }

// Persisted returns the saved originals for Persistence.Save and Load.
func (p *Plugin) Persisted() []any { return []any{&p.originals.byEntity} }

// RegisterBehavior hosts an Each or Every of Idling, run once for an entity whose last effect ended;
// call before Use.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		if err := p.idlers.Add(b); err != nil {
			return fmt.Errorf("%w in %s — it takes Each for Idling", err, p.Name())
		}
	}
	return nil
}
