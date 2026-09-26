package climate

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/sky"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Plugin is the climate of a world: where in the world it lies, and its weather going from one of
// the climate's weathers to the next, saved with the game, the world's weather — wind, clouds,
// rain, snow, the temperature — following it.
type Plugin struct {
	cfg         Config
	worldPlugin *world.Plugin
	module      *module
	change      control.Queue[Change]
	set         control.Queue[Set]
	report      report
	behaviours  host.EachHost[Weathering]
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin puts worldPlugin in the climate of cfg: it has skyPlugin's sun go the way it goes at
// the zone's latitude, and lets the climate's weather go by in the sky's time.
func NewPlugin(worldPlugin *world.Plugin, skyPlugin *sky.Plugin, cfg Config) *Plugin {
	cfg = cfg.withDefaults()
	skyPlugin.SetLatitude(cfg.Zone.Latitude)
	p := &Plugin{cfg: cfg, worldPlugin: worldPlugin}
	for _, s := range cfg.Weathers {
		p.report.names = append(p.report.names, s.Name)
	}
	return p
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.climate" }

// Install wires the weather's system.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{sys: newWeatherSystem(p.cfg, p.worldPlugin, &p.change, &p.set, &p.behaviours)}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan moves the weather on; call it after the sky's, before the world is drawn.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op: what falls is drawn in plain colours.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is what falls — rain, snow — a render.Source for a scene's Composer.
func (p *Plugin) Renderer() render.Layer { return &precipitation{world: p.worldPlugin} }

// Reporter is the weather's line for a render.TelemetryRenderer.
func (p *Plugin) Reporter() render.Reporter { return &p.report }

// EventHandler is nil: the weather takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the weather is the plugin's entity, saved with the ECS.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// RegisterBehavior hosts an Every of Weathering, run every tick with the weather; call before Use.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		if err := p.behaviours.Add(b); err != nil {
			return fmt.Errorf("%w in %s — it takes Every for Weathering", err, p.Name())
		}
	}
	return nil
}
