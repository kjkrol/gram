package world

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/kjkrol/aabbworld/geom"

	"github.com/kjkrol/aabbworld"
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
	renderer *renderer
	kinds    *Kinds
	roster   *kind.Roster
	seeded   []kind.Entry
	view     *view.View // the camera's
	views    map[camera.Camera]*view.View
	cameras  Cameras
	look     Look
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
	m.moments.effects = m.effects
	m.plans = steps.NewPlans(m.clock.Time, m.clock.Entity, cfg.Seed, m.effects, &m.commands)
	p.roster.Unit.Default(comp.Marks[effect.States]())
	p.roster.Unit.Default(comp.Const(steering.Course{}))
	if err := m.commands.Carry(p.Queues()...); err != nil {
		panic(err)
	}
	return p
}

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
	ctx.UseModule(p.module)
	ctx.UseModule(p.module.effects.Module())
	ctx.Setup(p.kinds)
	return nil
}

// RunPlan runs world's tick — call it first from your own Stage.Update: the clock's commands and
// the views at once, and movement, the leavers and the effects in every step of the simulation.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) {
	p.module.RunPlan(ctx, d)
}

// Queues are the clock's, Despawn's, Apply's, Dispel's and the steering's (steering.Away, Toward,
// Turn) — for the players plugin, which carries the world's commands itself.
func (p *Plugin) Queues() []control.CommandQueue {
	q := append(p.module.clock.Queues(), &p.module.despawns, &p.module.applies, &p.module.dispels)
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

// WithRenderer builds this plugin's own entity renderer, drawing cam-relative sprites from atlas.
func (p *Plugin) WithRenderer(atlas render.AtlasSource) {
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

// Hook hosts rules (rule.On) of a Moving (every entity, before it moves), a Leaving (every tick
// an entity is Outside an open edge) and a clock.Moment (every step). Call before Use.
func (p *Plugin) Hook(rules ...rule.Rule) error {
	for _, b := range rules {
		var err error
		hosts := []func(any) error{p.module.movers.Add, p.module.leavers.Add, p.module.moments.host.Add}
		for _, add := range hosts {
			if err = add(b); err == nil || !errors.Is(err, plugin.ErrUnhosted) {
				break
			}
		}
		if err != nil {
			return fmt.Errorf("%w in %s — it takes a rule of Moving, Leaving or clock.Moment", err, p.Name())
		}
	}
	return nil
}

// Draw has the world's renderer draw its entities as rules say, every frame, in the order given
// (render.Over, As, With, Show; Facing); call before Use.
func (p *Plugin) Draw(rules ...render.Rule) error {
	if err := p.module.drawing.Add(rules...); err != nil {
		return fmt.Errorf("%w in %s", err, p.Name())
	}
	return nil
}

// =================================================================
// world-specific
// =================================================================

// Seed adds entries to the entities spawned when this Stage starts fresh — see Populate.
func (p *Plugin) Seed(entries ...kind.Entry) { p.seeded = append(p.seeded, entries...) }

// Populate spawns every seeded entity, or none and an error on an unknown kind or a wrong row.
func (p *Plugin) Populate() error {
	var order []string
	groups := make(map[string][]any)
	for _, e := range p.seeded {
		r, ok := p.kinds.r.Kind(e.Kind())
		if !ok {
			return fmt.Errorf("world: unknown kind %q", e.Kind())
		}
		if got := reflect.TypeOf(e.Row()); got != r.Row {
			return fmt.Errorf("world: kind %q: an entry carries a %v, its rows are %v", r.Name, got, r.Row)
		}
		if _, seen := groups[r.Name]; !seen {
			order = append(order, r.Name)
		}
		groups[r.Name] = append(groups[r.Name], e.Row())
	}
	for _, name := range order {
		k, _ := p.kinds.r.Kind(name)
		p.module.populate(k, groups[name])
	}
	p.seeded = nil
	return nil
}

// Despawn takes an entity out of the ECS at the end of the tick.
func (p *Plugin) Despawn(cb *goke.CmdBuf, id uid.UID64) { p.module.despawn(cb, id) }

// Space returns world's shared space, rebuilt from every entity each tick after movement.
func (p *Plugin) Space() *aabbworld.Space { return p.module.space }

// Kinds returns this Plugin's registry of entity kinds — what kind.Define registers with.
func (p *Plugin) Kinds() *Kinds { return p.kinds }
