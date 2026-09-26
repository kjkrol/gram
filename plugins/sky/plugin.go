package sky

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Plugin is a day going by over a world with heights: its time of day saved with the game, the
// world's sun following it, and commands to stop it, hurry it on or hold it back.
type Plugin struct {
	cfg         Config
	worldPlugin *world.Plugin
	module      *module
	forward     control.Queue[Forward]
	back        control.Queue[Back]
	pause       control.Queue[Pause]
	clock       clock
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin lets a day of cfg go by over worldPlugin, lighting it by the sun of the hour.
func NewPlugin(worldPlugin *world.Plugin, cfg Config) *Plugin {
	return &Plugin{cfg: cfg.withDefaults(), worldPlugin: worldPlugin}
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.sky" }

// Install wires the sky's system.
func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{sys: newSkySystem(p.cfg, p.worldPlugin, &p.forward, &p.back, &p.pause)}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan moves the day on; call it before the world is drawn.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op: the sky is drawn in plain colours, and as the light on the world.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is the sky behind the world, a render.Source for a scene's Composer: the viewport in
// the sky's colour under everything, wherever the ground does not cover it.
func (p *Plugin) Renderer() render.Layer { return &backdrop{world: p.worldPlugin} }

// Reporter is the sky's line for a render.TelemetryRenderer: the time of day.
func (p *Plugin) Reporter() render.Reporter { return &p.clock }

// EventHandler is nil: the sky takes commands, not input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the time of day is the sky's entity, saved with the ECS.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// RegisterBehavior refuses every behavior: the sky hosts none.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		return fmt.Errorf("%w: %T in %s", plugin.ErrUnhostedBehavior, b, p.Name())
	}
	return nil
}

// =================================================================
// plugin.CommandHandler contract
// =================================================================

// Queues are where Forward, Back and Pause land.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.forward, &p.back, &p.pause}
}

// DefaultBindings: P stops the day or lets it go on; ] hurries it on — twice the pace, or half an
// hour later while it stands — and [ holds it back — half the pace, or half an hour earlier.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Command(control.KeyPress{Key: ebiten.KeyP}, "Stop the day, or let it go on", func(control.Context) (Pause, bool) { return Pause{}, true }),
		control.Command(control.KeyPress{Key: ebiten.KeyBracketRight}, "Hurry the day on", func(control.Context) (Forward, bool) { return Forward{}, true }),
		control.Command(control.KeyPress{Key: ebiten.KeyBracketLeft}, "Hold the day back", func(control.Context) (Back, bool) { return Back{}, true }),
	}
}
