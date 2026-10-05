package world

import (
	"fmt"
	"reflect"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	ilook "github.com/kjkrol/gram/plugins/world/internal/look"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Resources is world's single published Resources.
type Resources struct {
	Config    Config
	Telemetry *Telemetry
	Camera    camera.Camera
}

var _ plugin.Serializable = (*Resources)(nil)

// Persisted returns the camera's Viewport/Zoom for Persistence.Save/Load to include automatically.
func (r *Resources) Persisted() []any { return r.Camera.Persisted() }

// Plugin is the world a Stage installs via ctx.UseWorld — never construct
// and Use your own.
type Plugin struct {
	Res      Resources
	module   *module
	roles    Roles
	plans    Plans
	*Self    // the world's own entity, the clock's: what entity.World names
	renderer *renderer
	kinds    *Kinds
	roster   *kind.Roster
	seeded   []kind.Entry
	view     *view.View // the camera's
	views    map[camera.Camera]*view.View
	cameras  Cameras
	look     Look
	looked   bool // the effects' looks are among the drawing rules
	sections any  // the Stage's Initializer, which tells the section being defined; nil before Install
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Builtin = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)
var _ plugin.Populator = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// Builtin marks Plugin as installed by the engine itself — see ctx.UseWorld.
func (*Plugin) Builtin() {}

// NewPlugin builds Plugin around a fresh world, usable before Install.
func NewPlugin(cfg Config) *Plugin {
	m := newModule(cfg)
	kinds := newKinds(cfg.Heights)
	m.kinds = kinds
	p := &Plugin{Res: Resources{Config: cfg, Telemetry: &m.telemetry}, module: m, kinds: kinds, roster: kind.NewRoster(),
		cameras: icamera.NewFromSpaceWithConfig,
		look:    ilook.NewFlat(float32(cfg.Space.Width), float32(cfg.Space.Height))}
	p.Res.Camera = p.NewCamera()
	p.view = p.newView(p.Res.Camera.Bounds)
	kind.Require[Position](&p.roster.Unit, "world", "where it stands")
	p.roster.Unit.Default(comp.Const(Velocity{}))
	m.effects = effect.New(func(name string) tag.Tag[effect.States] { return kinds.DefineTag[effect.States](name) })
	m.effects.Sprites(kinds.NewSprite)
	m.effects.Guard(func(name string) { p.must(fmt.Sprintf("effect %q defined", name), section.Effects) })
	kinds.guard = func(name string) { p.must(fmt.Sprintf("kind %q defined", name), section.Kinds) }
	m.castings.effects, m.castings.world, m.castings.commands = m.effects, m.clock.Entity, &m.commands
	m.plans = steps.NewPlans(m.clock.Time, m.clock.Entity, cfg.Seed, m.effects, &m.commands)
	m.plans.Roles(m.castings.RolesOf)
	p.roster.Unit.Default(comp.Marks[effect.States]())
	p.roster.Unit.Default(comp.Const(steering.Course{}))
	if err := m.commands.Carry(p.Queues()...); err != nil {
		panic(err)
	}
	p.roles.w, p.plans.w = p, p
	p.Self = NewSelf(p, p.Name(), comp.Const(m.clock.State())) // the clock's entity is the world's own
	return p
}

// InSection is an error for what when the Stage, built in sections (package game/stage), is
// defining another part than those want names; nil for a Stage written by hand. For plugins,
// which refuse what is defined out of its place.
func (p *Plugin) InSection(what string, want ...section.Part) error {
	return section.Check(p.sections, what, want...)
}

// must panics with what InSection says.
func (p *Plugin) must(what string, want ...section.Part) {
	if err := p.InSection(what, want...); err != nil {
		panic("world: " + err.Error())
	}
}

// Roles are the roles of this Stage, by name: where a game defines them and finds them again.
func (p *Plugin) Roles() *Roles { return &p.roles }

// Plans are the plans of this Stage, by name: where a game defines them and finds them again.
func (p *Plugin) Plans() *Plans { return &p.plans }

// Roster is what this world's plugins ask of the kinds a game defines; build a unit's Spec through
// Roster().Unit.Spec.
func (p *Plugin) Roster() *kind.Roster { return p.roster }

// Clock is the world's tactical clock: the game time everything that simulates goes by, its
// tactical pause and its tempo. A plugin hands it what simulates (clock.Clock.Simulate).
func (p *Plugin) Clock() *clock.Clock { return p.module.clock }

// Effects are the world's effects: temporary changes to entities, counted down in the clock's time.
func (p *Plugin) Effects() *effect.Effects { return p.module.effects }

// Tick is the Tick a plugin hands the rules it hosts for a pass over d of the simulation: the
// world's carrier of commands, the game time the step ends at and the world's seed.
func (p *Plugin) Tick(cb *goke.CmdBuf, d time.Duration) plugin.Tick { return p.module.tick(cb, d) }

// HasHeights reports whether this world has heights — see Config.Heights.
func (p *Plugin) HasHeights() bool { return p.Res.Config.Heights }

// View is what the camera sees: refreshed each tick after movement, drawn by the entity renderer.
func (p *Plugin) View() *view.View { return p.view }

// newView keeps a View current over whatever bounds says, from the next tick on — a second
// camera's, a remote player's, anything that watches a part of the world.
func (p *Plugin) newView(bounds func() geom.AABB) *view.View {
	v := view.New(bounds)
	p.module.views = append(p.module.views, v)
	return v
}

// NewCamera is another camera over this world, configured as the world's own — a player's who
// looks on their own; ViewFor gives its View.
func (p *Plugin) NewCamera() camera.Camera {
	cfg := p.Res.Config
	return p.cameras(cfg.Space.Width, cfg.Space.Height, cfg.Space.Edges, cfg.Camera)
}

// SetCameras has the world make its cameras with make from now on, its own camera anew: a view
// plugin's projection. Call before anything asks for a camera — right after the world is made.
func (p *Plugin) SetCameras(make Cameras) {
	p.cameras = make
	p.dropView(p.view)
	p.Res.Camera = p.NewCamera()
	p.view = p.newView(p.Res.Camera.Bounds)
}

// Scale is how many metres a world unit spans, as the world was made with.
func (p *Plugin) Scale() Scale { return p.Res.Config.Scale }

// SetLook has the world's entities drawn, picked and outlined by look: a view plugin's.
func (p *Plugin) SetLook(look Look) { p.look = look }

// Look is how the world's entities lie on the screen.
func (p *Plugin) Look() Look { return p.look }

// FlatLook is the world seen from above, its own Look before any view set another: each entity's
// sprite over its box, split at a wrap seam.
func (p *Plugin) FlatLook() Look {
	return ilook.NewFlat(float32(p.Res.Config.Space.Width), float32(p.Res.Config.Space.Height))
}

// ViewFor is the View of what cam sees, kept current from the next tick on: View for the world's
// camera, one made at the first call for any other.
func (p *Plugin) ViewFor(cam camera.Camera) *view.View {
	if cam == p.Res.Camera {
		return p.view
	}
	if v, ok := p.views[cam]; ok {
		return v
	}
	if p.views == nil {
		p.views = map[camera.Camera]*view.View{}
	}
	v := p.newView(cam.Bounds)
	p.views[cam] = v
	return v
}

// dropView stops refreshing v; it keeps whatever it last saw.
func (p *Plugin) dropView(v *view.View) {
	views := p.module.views
	for i, w := range views {
		if w == v {
			p.module.views = append(views[:i], views[i+1:]...)
			return
		}
	}
}

// Camera returns world's shared Camera, built from Config.Space.
func (p *Plugin) Camera() camera.Camera { return p.Res.Camera }

// Restore applies the camera's Viewport/Zoom decoded by Persistence.Load.
func (p *Plugin) Restore() { p.Res.Camera.Restore() }

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.world" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.sections = ctx
	ctx.UseModule(p.module)
	ctx.UseModule(p.module.effects.Module())
	ctx.Setup(p.kinds)
	ctx.Hosts(p.module.movers, p.module.leavers, &p.module.moments.host)
	return nil
}

// RunPlan runs world's tick — call it from your own Stage.Update before whatever reads the
// world's space, after only what moves entities itself (bullet): the clock's commands and the
// views at once, and movement, the leavers and the effects in every step of the simulation.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.module.RunPlan(ctx, d)
}

// Queues are the clock's, Spawn's, Despawn's, the commands' about effects (rule.Casting,
// rule.Triggered) and the steering's (steering.Away, Toward, Turn) — for the players plugin, which
// carries the world's commands itself.
func (p *Plugin) Queues() []control.CommandQueue {
	q := append(p.module.clock.Queues(), &p.module.spawns, &p.module.despawns, &p.module.castings.queue, &p.module.castings.triggers)
	return append(q, p.module.steer.Queues()...)
}

// Carry has the world take commands — its players', its entities' (Order in a plan or a rule) — to the
// queues of handlers; the engine carries every plugin.CommandHandler it is given with Use.
func (p *Plugin) Carry(handlers ...plugin.CommandHandler) error {
	for _, h := range handlers {
		if err := p.module.commands.Carry(h.Queues()...); err != nil {
			return err
		}
	}
	return nil
}

// Commands is what takes the commands the world's entities give themselves to their handlers:
// a host's plugin.Tick carries it.
func (p *Plugin) Commands() *control.Carrier { return &p.module.commands }

// DefaultBindings are the clock's: Space pauses, ] and [ set the tempo.
func (p *Plugin) DefaultBindings() []control.Binding { return p.module.clock.DefaultBindings() }

// WithRenderer builds this plugin's own entity renderer, drawing cam-relative sprites from atlas;
// the looks the effects were given (effect.Effect.Look) are swapped in after the rules of Draw.
func (p *Plugin) WithRenderer(atlas render.AtlasSource) {
	if !p.looked {
		p.looked = true
		p.module.effects.Looks(func(mark tag.Tag[effect.States], twins map[render.SpriteID]render.SpriteID) {
			if err := p.module.drawing.Add(render.Swap(twins, mark.In)); err != nil {
				panic(err)
			}
		})
	}
	p.renderer = newRenderer(atlas, p.ViewFor, &p.module.drawing, p.Look)
	p.renderer.clock = p.module.clock.Shown
}

// Renderer returns this plugin's own render.Renderer, or nil unless WithRenderer was called.
func (p *Plugin) Renderer() render.Layer {
	if p.renderer == nil {
		return nil
	}
	return p.renderer
}

// EventHandler returns nil — the camera is moved by players' Pan and Zoom commands.
func (p *Plugin) EventHandler() control.EventHandler { return nil }

// Serializable returns world's persistable state (its camera's Viewport/Zoom).
func (p *Plugin) Serializable() plugin.Serializable { return &p.Res }

// Draw has the world's renderer draw its entities as rules say, every frame, in the order given
// (render.Over, As, With, Show; Facing); call before Use.
func (p *Plugin) Draw(rules ...render.Rule) error {
	if err := p.InSection("drawing rules given", section.Looks); err != nil {
		return err
	}
	if err := p.module.drawing.Add(rules...); err != nil {
		return fmt.Errorf("%w in %s", err, p.Name())
	}
	return nil
}

// =================================================================
// world-specific
// =================================================================

// Seed adds entries to the entities spawned when this Stage starts fresh — see Populate.
func (p *Plugin) Seed(entries ...kind.Entry) {
	p.must("units seeded", section.Units)
	p.seeded = append(p.seeded, entries...)
}

// Populate spawns every seeded entity, or none and an error on an unknown kind or a wrong row.
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

// Spawn adds entities of their kinds to the running world, as the game's own (control.Nobody):
// the command [Spawn] for each, carried out at the world's next step of the simulation.
func (p *Plugin) Spawn(entries ...kind.Entry) {
	for _, e := range entries {
		p.module.spawns.Add(control.Nobody, Spawn{Entry: e})
	}
}

// Despawn takes an entity out of the ECS at the end of the tick.
func (p *Plugin) Despawn(cb *goke.CmdBuf, id uid.UID64) { p.module.despawn(cb, id) }

// Space returns world's shared space, rebuilt from every entity each tick after movement.
func (p *Plugin) Space() *aabbworld.Space { return p.module.space }

// Kinds returns this Plugin's registry of entity kinds — what kind.Define registers with.
func (p *Plugin) Kinds() *Kinds { return p.kinds }

// Triggers hands the world the game's commands about effects: it gives those with a Source when
// the entity it names Triggers, and checks the names they all say once the game stands. For the
// engine: a Stage gives them to its Initializer.
func (p *Plugin) Triggers(cmds ...rule.Casting) error { return p.module.castings.take(cmds...) }
