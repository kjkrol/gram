package collision

import (
	"log"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// Plugin wires collision into a Stage over the world's space.
type Plugin struct {
	*world.Self

	worldPlugin *world.Plugin
	motion      *motion.Plugin
	module      *module

	Res Resources

	pairs    host.PairRules[Meeting]
	entities host.Rules[Struck]
	field    Field
	stats    *ContactStats
	log      *log.Logger

	boxes     command.Queue[Boxes]
	boxesView *boxesView
	kept      *plugin.Kept[Settings]
	read      Read
	write     Write
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.KindTemplater = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Tunable = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)

// NewPlugin builds the collision plugin over worldPlugin's space, its leavers motion's.
func NewPlugin(worldPlugin *world.Plugin, m *motion.Plugin) *Plugin {
	p := &Plugin{worldPlugin: worldPlugin, motion: m}
	p.read.p, p.write.p = p, p
	p.Self = world.NewSelf(worldPlugin, p)
	p.boxesView = &boxesView{w: worldPlugin, settings: &p.Res.Settings}
	p.kept = plugin.Keep(&p.Res.Settings)
	return p
}

// KindTemplates is what collision asks of the kinds a game defines: every unit a collider of the
// physics' defaults.
func (p *Plugin) KindTemplates(r *kind.Roster) {
	r.Default(motion.UnitTemplate, comp.Const(Collider{}))
	r.Default(motion.UnitTemplate, comp.Const(Physics{}))
}

func (p *Plugin) Name() string { return "gram.collision" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = newModule(p.worldPlugin.Write().Space(), ctx.ECS(), &p.pairs, &p.entities)
	p.module.fieldOf, p.module.clock = func() Field { return p.field }, p.worldPlugin.Write().Clock()
	p.module.heights = p.worldPlugin.Read(0).HasHeights()
	p.module.stats, p.module.log = p.stats, p.log
	p.module.tick = p.worldPlugin.Read(0).Tick
	p.module.pace = p.worldPlugin.Pace(sector.Near)
	p.module.leave = p.motion.Write().Leave
	ctx.UseModule(p.module)
	ctx.Hosts(&p.pairs, &p.entities)
	ctx.Draws(p.boxesView)
	return nil
}

// WithStats has collision count every contact into s; call before Use.
func (p *Plugin) WithStats(s *ContactStats) *Plugin {
	p.stats = s
	return p
}

// WithLog has collision write a line to l for every contact between entities; call before Use.
func (p *Plugin) WithLog(l *log.Logger) *Plugin {
	p.log = l
	return p
}

// WithField makes f the solid ground colliders are pushed out of; call before Use.
func (p *Plugin) WithField(f Field) *Plugin {
	p.field = f
	return p
}

// RunPlan runs the collisions in the simulation; call it after motion's RunPlan.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.switchBoxes()
	p.module.RunPlan(ctx, d)
}

// EventHandler is nil: collision reads no input.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is collision's Settings, kept whole; its components go with the entities.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Settings loaded.
func (p *Plugin) Restore() { p.kept.Restore() }
