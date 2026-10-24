package world

import (
	"fmt"
	cgoke "github.com/kjkrol/ceremony/goke"
	"image/color"
	"reflect"
	"time"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// Resources is what the world publishes: its Config, its Telemetry and its Settings.
type Resources struct {
	Config    Config
	Telemetry *Telemetry
	Settings  Settings
}

// Plugin is the world a Stage makes with its World link (ctx.UseWorld); never Use your own.
type Plugin struct {
	Res     Resources
	kept    *plugin.Kept[Settings]
	module  *module
	figures []Figure
	roles   Roles
	named   Commands
	made    plugin.Made
	*Self
	renderer *renderer
	sectors  *sectorsView
	kinds    *Kinds
	roster   *kind.Roster
	seeded   []kind.Entry
	views    map[camera.Camera]*view.View
	look     Look
	looked   bool
	styles   *render.Styles
	ctx      plugin.Installer
	picture  *render.Composer
	lights   func() *render.Lights
	read     Read
	reads    parallel.Reads[*Read]
	write    Write
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Builtin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.KindTemplater = (*Plugin)(nil)
var _ plugin.Tunable = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)

// Builtin marks Plugin as installed by the engine itself.
func (*Plugin) Builtin() {}

// NewPlugin makes the world from cfg, usable before Install.
func NewPlugin(cfg Config) *Plugin {
	m := newModule(cfg)
	p := &Plugin{
		Res:    Resources{Config: cfg, Telemetry: &m.telemetry},
		module: m,
		kinds:  newKinds(cfg.Heights),
		look:   flatLook(cfg.Space),
	}
	p.roster = kind.NewRoster(p.fillRoster)
	p.kept = plugin.Keep(&p.Res.Settings)
	m.p, m.kinds = p, p.kinds
	p.newEffects()
	p.newPlans()
	p.carryOwn()
	p.read, p.write = Read{p: p, clock: m.clock.Read(), effects: m.effects.Read()}, Write{p: p}
	p.reads = parallel.Reads[*Read]{&p.read}
	// the clock's entity is the world's own
	p.Self = newSelf(p, p.Name(), comp.Const(m.clock.Read().State()))
	p.renderer = newRenderer(nil, func() map[camera.Camera]*view.View { return p.views }, &m.drawing,
		p.read.Look)
	p.sectors = newSectorsView(p)
	p.renderer.clock = m.clock.Read().Shown
	return p
}

// Read reads the world's state on worker w of the game's pool, 0 the game's own goroutine; the
// other workers' reads come with Install.
func (p *Plugin) Read(w int) *Read { return p.reads[w] }

// Write changes the world's state, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// KindTemplates is what the world asks of the kinds a game defines: of every one it spawns,
// where it stands, its velocity and its effects' markers.
func (p *Plugin) KindTemplates(r *kind.Roster) {
	kind.Require[Position](r, EntityTemplate, "world", "where it stands")
	r.Default(EntityTemplate, comp.Const(Velocity{}))
	r.Default(EntityTemplate, comp.Marks[effect.States]())
}

// fillRoster has the world and every plugin made over it, in the order made, write what they ask
// of the kinds' templates
func (p *Plugin) fillRoster(r *kind.Roster) {
	p.KindTemplates(r)
	for _, made := range p.made.All() {
		if t, ok := made.(plugin.KindTemplater); ok {
			t.KindTemplates(r)
		}
	}
}

// newEffects makes the register of effects, their tags the kinds', and guards it and the kinds'
// to their sections
func (p *Plugin) newEffects() {
	m, kinds := p.module, p.kinds
	m.effects = cgoke.NewEffects(effect.New(func(name string) tag.Tag[effect.States] {
		return kinds.DefineTag[effect.States](name)
	}), p.Pace(sector.Paced))
	m.effects.Guard(func(name string) {
		p.must(fmt.Sprintf("effect %q defined", name), section.Effects)
	})
	kinds.guard = func(name string) { p.must(fmt.Sprintf("kind %q defined", name), section.Kinds) }
}

// newPlans makes the commands about effects and the plans, which play the roles they say
func (p *Plugin) newPlans() {
	m := p.module
	m.effectCmds.effects, m.effectCmds.world, m.effectCmds.commands = m.effects, m.clock.Read().Entity,
		&m.commands
	m.plans = plan.NewPlans(m.clock.Read().Time, p.Res.Config.Seed, m.effects, &m.commands)
	m.plansSys = cgoke.NewPlans(m.plans, p.Pace(sector.Paced))
	m.plans.Roles(m.effectCmds.RolesOf)
	p.kinds.plans = m.plans
}

// carryOwn has the world's carrier take the world's own commands, and its registers of roles and
// commands answer for it
func (p *Plugin) carryOwn() {
	if err := p.module.commands.Carry(p.Queues()...); err != nil {
		panic(err)
	}
	p.roles.w, p.named.w = p, p
}

// Made are the plugins made over the world, in the order made, which the Stage uses in that
// order.
func (p *Plugin) Made() *plugin.Made { return &p.made }

// InSection is an error for what defined while the Stage is in another section than those
// want names; nil for a Stage written by hand.
func (p *Plugin) InSection(what string, want ...section.Part) error {
	return section.Check(p.ctx, what, want...)
}

func (p *Plugin) must(what string, want ...section.Part) {
	if err := p.InSection(what, want...); err != nil {
		panic("world: " + err.Error())
	}
}

// Roles is this Stage's register of roles, by name.
func (p *Plugin) Roles() *Roles { return &p.roles }

// Roster is what this world's plugins ask of the kinds a game defines.
func (p *Plugin) Roster() *kind.Roster { return p.roster }

// WithFog has what lies far off fade into c where nothing else, the atmosphere, gives the fog's
// colour; a scene sets it as it begins.
func (p *Plugin) WithFog(c color.RGBA) *Plugin {
	p.renderer.fog = &[3]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255}
	return p
}

func flatLook(space SpaceCfg) *ilook.Flat {
	return ilook.NewFlat(float32(space.Width), float32(space.Height), space.Edges.WrapsX(),
		space.Edges.WrapsY())
}

func (p *Plugin) Name() string { return "gram.world" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.ctx = ctx
	ctx.UseModule(p.module)
	ctx.UseModule(p.module.effects.Module())
	ctx.Setup(p.kinds)
	ctx.Hosts(&p.module.moments.host)
	ctx.Draws(p.renderer, p.sectors)
	ctx.Looks(p.dress)
	p.reads = parallel.NewReads(&p.read, ctx.Pool())
	return nil
}

// dress declares the figures' looks on the world's atlas and closes it, unless a game closed its
// own, and makes the picture of what the Stage's plugins draw
func (p *Plugin) dress() {
	if !p.looked {
		p.NewAtlas().Looks().Close()
	}
	d, ok := p.ctx.(interface{ Drawn() []render.Source })
	if !ok {
		panic("world: a picture is made of what the Stage's plugins draw, and none was told")
	}
	p.picture = render.NewComposer(d.Drawn()...)
	p.picture.Light(p.lights)
}

// RunPlan runs the world's tick. Call it from Stage.Update before whatever reads the world's
// space, after only what moves entities itself.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.module.RunPlan(ctx, d)
}

// Carry has the world's carrier take commands to the queues of handlers.
func (p *Plugin) Carry(handlers ...plugin.CommandHandler) error {
	for _, h := range handlers {
		if err := p.module.commands.Carry(h.Queues()...); err != nil {
			return err
		}
	}
	return nil
}

// EventHandler is nil: a player's bindings issue the world's commands.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is the world's Settings, kept whole; its entities are saved with the ECS.
func (p *Plugin) Serializable() plugin.Serializable { return p.kept }

// Restore takes the Settings loaded and draws as they say.
func (p *Plugin) Restore() {
	p.kept.Restore()
	p.write.ShowSectors(p.Res.Settings.Sectors)
	p.write.SetBanded(!p.Res.Settings.Unbanded)
}

// Populate spawns every seeded entity, or none with an error on an unknown kind or a wrong row.
func (p *Plugin) Populate() error {
	var order []string
	groups := make(map[string][]any)
	entries := make(map[string][]kind.Entry)
	for _, e := range p.seeded {
		r, ok := p.kinds.r.Kind(e.Kind())
		if !ok {
			return fmt.Errorf("world: unknown kind %q", e.Kind())
		}
		if got := reflect.TypeOf(e.Row()); got != r.Row {
			return fmt.Errorf("world: kind %q: an entry carries a %v, its rows are %v", r.Name, got, r.Row)
		}
		if err := p.module.handled(e); err != nil {
			return fmt.Errorf("world: kind %q: %w", r.Name, err)
		}
		if _, seen := groups[r.Name]; !seen {
			order = append(order, r.Name)
		}
		groups[r.Name] = append(groups[r.Name], e.Row())
		entries[r.Name] = append(entries[r.Name], e)
	}
	for _, name := range order {
		k, _ := p.kinds.r.Kind(name)
		p.module.populateEntries(k, groups[name], entries[name])
	}
	p.seeded = nil
	return nil
}

// Kinds is this Stage's register of entity kinds, which kind.Define registers with.
func (p *Plugin) Kinds() *Kinds { return p.kinds }

// Commands is this Stage's register of commands about effects, by name.
func (p *Plugin) Commands() *Commands { return &p.named }
