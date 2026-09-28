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
	"github.com/kjkrol/gram/control"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world/clock"
	"github.com/kjkrol/gram/plugins/world/effects"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
	"github.com/kjkrol/gram/render"
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
	renderer *Renderer
	kinds    *Kinds
	roster   *kind.Roster
	ground   Ground
	cover    Cover
	field    Field
	seeded   []kind.Entry
	view     *View // the camera's
	views    map[camera.Camera]*View
	cameras  Cameras
	look     Look
	sun      Sun
	sunlit   bool // something set the sun: a flat world is lit by it too
	weather  Weather
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
	kinds := newKinds(cfg.Quasi3D)
	m.kinds = kinds
	p := &Plugin{Res: Resources{Config: cfg, Telemetry: &m.telemetry}, module: m, kinds: kinds, roster: kind.NewRoster(),
		cameras: icamera.NewFromSpaceWithConfig, sun: DefaultSun,
		look: &flatLook{worldW: float32(cfg.Space.Width), worldH: float32(cfg.Space.Height)}}
	p.Res.Camera = p.NewCamera()
	p.view = p.NewView(p.Res.Camera.Bounds)
	kind.Require[Position](&p.roster.Unit, "world", "where it stands")
	p.roster.Unit.Default(comp.Const(Velocity{}))
	return p
}

// Roster is what this world's plugins ask of the kinds a game defines; build a unit's Spec through
// Roster().Unit.Spec.
func (p *Plugin) Roster() *kind.Roster { return p.roster }

// Clock is the world's tactical clock: the game time everything that simulates goes by, its
// tactical pause and its tempo. A plugin hands it what simulates (clock.Clock.Simulate).
func (p *Plugin) Clock() *clock.Clock { return p.module.clock }

// Effects are the world's effects and their schedule: temporary changes to entities, counted down
// in the clock's time.
func (p *Plugin) Effects() *effects.Effects { return p.module.effects }

// Quasi3D reports whether this world has heights — see Config.Quasi3D.
func (p *Plugin) Quasi3D() bool { return p.Res.Config.Quasi3D }

// SetGround gives a Quasi3D world its ground heights; the board calls it, sight reads Ground.
func (p *Plugin) SetGround(g Ground) { p.ground = g }

// Ground is the world's ground heights, nil for flat ground at 0.
func (p *Plugin) Ground() Ground { return p.ground }

// SetCover gives the world the cover standing on its ground; the board calls it, sight reads Cover.
func (p *Plugin) SetCover(c Cover) { p.cover = c }

// Cover is the cover standing on the world's ground, nil for none.
func (p *Plugin) Cover() Cover { return p.cover }

// SetField gives the world its solid ground; the board calls it, collisions read Field.
func (p *Plugin) SetField(f Field) { p.field = f }

// Field is the world's solid ground, nil for none.
func (p *Plugin) Field() Field { return p.field }

// View is what the camera sees: refreshed each tick after movement, drawn by the entity renderer.
func (p *Plugin) View() *View { return p.view }

// NewView keeps a View current over whatever bounds says, from the next tick on — a second
// camera's, a remote player's, anything that watches a part of the world.
func (p *Plugin) NewView(bounds func() geom.AABB) *View {
	v := newView(bounds)
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
	p.DropView(p.view)
	p.Res.Camera = p.NewCamera()
	p.view = p.NewView(p.Res.Camera.Bounds)
}

// SetSun lights the world with sun from now on — a game's, or the sky of a day going by. From
// then on a flat world is Sunlit too.
func (p *Plugin) SetSun(sun Sun) { p.sun, p.sunlit = sun, true }

// Sunlit reports whether the Sun lights what is drawn: always in a world with heights, whose
// ground it shades; in a flat one once something set it (SetSun) — the sky of a day going by
// tinting the tiles and the sprites, night dark, dawn warm — and never before, its sprites drawn
// as they are.
func (p *Plugin) Sunlit() bool { return p.Res.Config.Quasi3D || p.sunlit }

// Sun is the world's light: DefaultSun unless something set another.
func (p *Plugin) Sun() Sun { return p.sun }

// SetWeather has the air over the world be w from now on — a game's, or the weather going by.
func (p *Plugin) SetWeather(w Weather) { p.weather = w }

// Weather is the air over the world: a calm, clear day unless something set another.
func (p *Plugin) Weather() Weather { return p.weather }

// SetLook has the world's entities drawn, picked and outlined by look: a view plugin's.
func (p *Plugin) SetLook(look Look) { p.look = look }

// Look is how the world's entities lie on the screen.
func (p *Plugin) Look() Look { return p.look }

// FlatLook is the world seen from above, its own Look before any view set another: each entity's
// sprite over its box, split at a wrap seam.
func (p *Plugin) FlatLook() Look {
	return &flatLook{worldW: float32(p.Res.Config.Space.Width), worldH: float32(p.Res.Config.Space.Height)}
}

// ViewFor is the View of what cam sees, kept current from the next tick on: View for the world's
// camera, one made at the first call for any other.
func (p *Plugin) ViewFor(cam camera.Camera) *View {
	if cam == p.Res.Camera {
		return p.view
	}
	if v, ok := p.views[cam]; ok {
		return v
	}
	if p.views == nil {
		p.views = map[camera.Camera]*View{}
	}
	v := p.NewView(cam.Bounds)
	p.views[cam] = v
	return v
}

// DropView stops refreshing v; it keeps whatever it last saw.
func (p *Plugin) DropView(v *View) {
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
	p.module.ecs = ctx.ECS()
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

// Queues are the clock's — for the players plugin, which carries the world's commands itself.
func (p *Plugin) Queues() []control.CommandQueue { return p.module.clock.Queues() }

// DefaultBindings are the clock's: Space pauses, ] and [ set the tempo.
func (p *Plugin) DefaultBindings() []control.Binding { return p.module.clock.DefaultBindings() }

// WithRenderer builds this plugin's own entity renderer, drawing cam-relative sprites from atlas.
func (p *Plugin) WithRenderer(atlas render.AtlasSource) {
	p.renderer = newRenderer(atlas, p.ViewFor, p.module.drawers, p.Look)
	p.renderer.sun, p.renderer.sunlit = p.Sun, p.Sunlit
	if p.Quasi3D() {
		p.renderer.ground = p.Ground
	}
	p.renderer.weather = p.Weather
	p.renderer.clock = p.module.clock.Time
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

// RegisterBehavior adds world.Behaviors to the decision pass run before movement, in order, and
// hosts Each and Every of a Moving (every entity, before it moves), a Leaving (every tick an
// entity is Outside an open edge), a Drawing (every entity about to be drawn) and an
// effects.Idling (an entity whose last effect ended). Call before Use.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		if system, ok := b.(Behavior); ok {
			p.module.RegisterBehavior(system)
			continue
		}
		var err error
		hosts := []func(plugin.Behavior) error{p.module.movers.Add, p.module.leavers.Add, p.module.drawers.Add, p.module.effects.Host}
		for _, add := range hosts {
			if err = add(b); err == nil || !errors.Is(err, plugin.ErrUnhostedBehavior) {
				break
			}
		}
		if err != nil {
			return fmt.Errorf("%w in %s — it takes a world.Behavior or Each/Every for Moving, Leaving, Drawing or Idling", err, p.Name())
		}
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
		r, ok := p.kinds.entries[e.Kind()]
		if !ok {
			return fmt.Errorf("world: unknown kind %q", e.Kind())
		}
		if got := reflect.TypeOf(e.Row()); got != r.row {
			return fmt.Errorf("world: kind %q: an entry carries a %v, its rows are %v", r.name, got, r.row)
		}
		if _, seen := groups[r.name]; !seen {
			order = append(order, r.name)
		}
		groups[r.name] = append(groups[r.name], e.Row())
	}
	for _, name := range order {
		p.module.populate(p.kinds.entries[name], groups[name])
	}
	p.seeded = nil
	return nil
}

// Attach gives id the component v from the next sync on, replacing one it already carries.
func (p *Plugin) Attach[T any](cb *goke.CmdBuf, id uid.UID64, v T) {
	cb.AddOne(id, compID[T](p), v)
}

// Detach takes T off id from the next sync on; an entity without it is left alone.
func (p *Plugin) Detach[T any](cb *goke.CmdBuf, id uid.UID64) {
	if reflect.TypeFor[T]() == reflect.TypeFor[Base]() {
		panic("world: Detach[Base] — every entity carries a Base; Despawn the entity instead")
	}
	cb.RemoveCompOne(id, compID[T](p))
}

// Declare tells save files about T, a component only ever attached; call it in Stage.Init.
func (p *Plugin) Declare[T any]() {
	p.module.declared = append(p.module.declared, goke.LoadComp[T]())
}

func compID[T any](p *Plugin) goke.CompID {
	if p.module.ecs == nil {
		panic("world: Attach/Detach before the Plugin was installed")
	}
	return p.module.ecs.RegComp[T]()
}

// Despawn takes an entity out of the ECS at the end of the tick.
func (p *Plugin) Despawn(cb *goke.CmdBuf, id uid.UID64) { p.module.despawn(cb, id) }

// Space returns world's shared space, rebuilt from every entity each tick after movement.
func (p *Plugin) Space() *aabbworld.Space { return p.module.space }

// Kinds returns this Plugin's registry of entity kinds — what kind.Define registers with.
func (p *Plugin) Kinds() *Kinds { return p.kinds }
