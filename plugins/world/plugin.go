package world

import (
	"fmt"
	cgoke "github.com/kjkrol/ceremony/goke"
	"image/color"
	"reflect"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Resources is what the world publishes: its Config and its Telemetry.
type Resources struct {
	Config    Config
	Telemetry *Telemetry
}

// Plugin is the world a Stage makes with its World link (ctx.UseWorld); never Use your own.
type Plugin struct {
	Res     Resources
	module  *module
	figures []Figure
	roles   Roles
	named   Commands
	made    plugin.Made
	*Self
	renderer *renderer
	kinds    *Kinds
	roster   *kind.Roster
	seeded   []kind.Entry
	views    map[camera.Camera]*view.View
	look     Look
	looked   bool
	styles   *render.Styles
	ctx      plugin.Installer
	picture  *render.Composer
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Builtin = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.KindTemplater = (*Plugin)(nil)

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
	m.kinds = p.kinds
	p.newEffects()
	p.newPlans()
	p.carryOwn()
	// the clock's entity is the world's own
	p.Self = newSelf(p, p.Name(), comp.Const(m.clock.State()))
	p.renderer = newRenderer(nil, p.ViewFor, &m.drawing, p.Look)
	p.renderer.clock = m.clock.Shown
	return p
}

// KindTemplates is what the world asks of the kinds a game defines: of every one it spawns,
// where it stands, its velocity, its effects' markers and a live patch of a shadow.
func (p *Plugin) KindTemplates(r *kind.Roster) {
	kind.Require[Position](r, EntityTemplate, "world", "where it stands")
	r.Default(EntityTemplate, comp.Const(Velocity{}))
	r.Default(EntityTemplate, comp.Marks[effect.States]())
	r.Default(EntityTemplate, comp.Const(Shadow{Cast: Patch}))
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
	m.effectCmds.effects, m.effectCmds.world, m.effectCmds.commands = m.effects, m.clock.Entity,
		&m.commands
	m.plans = plan.NewPlans(m.clock.Time, p.Res.Config.Seed, m.effects, &m.commands)
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

// Clock is the world's tactical clock, which everything that simulates goes by.
func (p *Plugin) Clock() *clock.Clock { return p.module.clock }

// Effects is this Stage's register of effects, which last in the clock's time.
func (p *Plugin) Effects() *cgoke.Effects { return p.module.effects }

// Tick is the Tick a plugin hands the rules it hosts for a pass over d of the simulation.
func (p *Plugin) Tick(d time.Duration) host.Tick { return p.module.tick(d) }

// HasHeights reports whether this world has heights.
func (p *Plugin) HasHeights() bool { return p.Res.Config.Heights }

// WithFog has what lies far off fade into c where nothing else, the atmosphere, gives the fog's
// colour; a scene sets it as it begins.
func (p *Plugin) WithFog(c color.RGBA) *Plugin {
	p.renderer.fog = &[3]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255}
	return p
}

// Scale is how many metres a world unit spans.
func (p *Plugin) Scale() Scale { return p.Res.Config.Scale }

// SetLook has the world's entities drawn, picked and outlined by look, a view plugin's.
func (p *Plugin) SetLook(look Look) { p.look = look }

// Look is how the world's entities lie on the screen.
func (p *Plugin) Look() Look { return p.look }

// FlatLook is the world's own Look, seen from above.
func (p *Plugin) FlatLook() Look {
	return flatLook(p.Res.Config.Space)
}

func flatLook(space SpaceCfg) *ilook.Flat {
	return ilook.NewFlat(float32(space.Width), float32(space.Height), space.Edges.WrapsX(),
		space.Edges.WrapsY())
}

// ViewFor is the View of what cam sees, made at the first call and kept current from the next
// tick on.
func (p *Plugin) ViewFor(cam camera.Camera) *view.View {
	if v, ok := p.views[cam]; ok {
		return v
	}
	if p.views == nil {
		p.views = map[camera.Camera]*view.View{}
	}
	v := view.New(cam.Bounds)
	if p.module.sectors.grid != nil {
		v = view.NewSeen(cam, seenSectors{p.module.sectors})
	}
	p.module.views = append(p.module.views, v)
	p.views[cam] = v
	return v
}

func (p *Plugin) Name() string { return "gram.world" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.ctx = ctx
	ctx.UseModule(p.module)
	ctx.UseModule(p.module.effects.Module())
	ctx.Setup(p.kinds)
	ctx.Hosts(&p.module.moments.host)
	ctx.Draws(p.renderer)
	ctx.Looks(p.dress)
	return nil
}

// Picture is everything the Stage's plugins draw, one picture for every scene showing the world.
func (p *Plugin) Picture() render.Picture {
	if p.picture == nil {
		d, ok := p.ctx.(interface{ Drawn() []render.Source })
		if !ok {
			panic("world: a picture is made of what the Stage's plugins draw, and none was told")
		}
		p.picture = render.NewComposer(d.Drawn()...)
	}
	return p.picture
}

// dress declares the figures' looks on the world's atlas and closes it, unless a game closed its
// own
func (p *Plugin) dress() {
	if !p.looked {
		p.NewAtlas().Looks().Close()
	}
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

// Carrier is the Stage's one carrier of commands, taking each to its handler's queue.
func (p *Plugin) Carrier() *command.Carrier { return &p.module.commands }

// SetAtlas takes atlas as the world's looks for the entity renderer, applying what a *Atlas
// declared; its Close calls it. A foreign render.AtlasSource is given by hand.
func (p *Plugin) SetAtlas(atlas render.AtlasSource) {
	if !p.looked {
		p.looked = true
		if wa, ok := atlas.(*Atlas); ok {
			if err := p.module.drawing.Add(wa.swaps()...); err != nil {
				panic(err)
			}
			if err := p.module.drawing.Add(wa.rules(p.module.clock.Shown)...); err != nil {
				panic(err)
			}
		}
	}
	p.renderer.atlas = atlas
	p.renderer.shade(nil, nil)
	p.renderer.lie(nil, nil, nil)
	if wa, ok := atlas.(*Atlas); ok {
		p.renderer.shade(wa.shaded, wa.phased)
		p.renderer.lie(wa.lain(), wa.edged(), wa.stood())
		p.styles = &wa.styles
	}
}

// Styles are the styles the figures declared on the world's atlas (Slot.Styled), for the
// plugins drawing about a kind; none before the atlas is closed.
func (p *Plugin) Styles() *render.Styles { return p.styles }

// EventHandler is nil: a player's bindings issue the world's commands.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable is nil: the world's entities are saved with the ECS.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// Seed adds entries to the entities spawned when this Stage starts fresh.
func (p *Plugin) Seed(entries ...kind.Entry) {
	p.must("units seeded", section.Spawn)
	p.seeded = append(p.seeded, entries...)
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

// Spawn gives the command [Spawn] for each entry as command.Nobody.
func (p *Plugin) Spawn(entries ...kind.Entry) {
	for _, e := range entries {
		p.module.spawns.Add(command.Nobody, Spawn{Entry: e})
	}
}

// Despawn takes an entity out of the ECS at the end of the tick.
func (p *Plugin) Despawn(cb *goke.CmdBuf, id uid.UID64) { p.module.despawn(cb, id) }

// Space is the world's shared spatial index of its entities, indexed again as they spawn, go and
// move.
func (p *Plugin) Space() *aabbworld.Space { return p.module.space }

// Kinds is this Stage's register of entity kinds, which kind.Define registers with.
func (p *Plugin) Kinds() *Kinds { return p.kinds }

// Commands is this Stage's register of commands about effects, by name.
func (p *Plugin) Commands() *Commands { return &p.named }
