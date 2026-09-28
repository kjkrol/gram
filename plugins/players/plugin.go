package players

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// ErrUnknownCommand is what Issue reports for a command type no handler defines.
var ErrUnknownCommand = errors.New("players: no plugin listens for this command")

// Plugin keeps the game's players and carries their commands to the handlers that define them: a
// player's bindings, an AI or a network issue a command, and it lands in its handler's queue.
type Plugin struct {
	worldPlugin *world.Plugin
	ground      func(x, y float32) float32 // the world's Ground for command contexts, bound at first use
	handlers    []plugin.CommandHandler
	owners      map[reflect.Type]plugin.CommandHandler // the handler of each command type
	players     []*Player
	queues      map[reflect.Type]control.CommandQueue
	pans        control.Queue[Pan]
	zooms       control.Queue[Zoom]
	module      *module
	layout      Layout
	// captured is whether the cursor is caught, as setCapture last set it; setCapture catches or
	// lets go of the window's cursor
	captured   bool
	setCapture func(on bool)
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)
var _ plugin.Restorer = (*Plugin)(nil)

// NewPlugin builds the players plugin over worldPlugin, whose camera and View the local players
// share, carrying the commands of handlers — the world's own, its clock's, among them; two
// handlers of one command type panic.
func NewPlugin(worldPlugin *world.Plugin, handlers ...plugin.CommandHandler) *Plugin {
	p := &Plugin{worldPlugin: worldPlugin, queues: map[reflect.Type]control.CommandQueue{}, owners: map[reflect.Type]plugin.CommandHandler{}}
	p.handlers = append([]plugin.CommandHandler{p, worldPlugin}, handlers...)
	for _, c := range p.handlers {
		for _, box := range c.Queues() {
			if other, taken := p.queues[box.Accepts()]; taken && other != box {
				panic(fmt.Sprintf("players: %v is defined twice", box.Accepts()))
			}
			p.queues[box.Accepts()], p.owners[box.Accepts()] = box, c
		}
	}
	return p
}

// Local adds a player at this keyboard, looking through the world's camera; bind it before Use.
func (p *Plugin) Local(name string) *Player {
	pl := p.Add(name)
	pl.local = true
	return pl
}

// Add adds a player without a keyboard — an AI, a remote client — whose commands come in through
// Issue; it looks through the world's camera until it has one of its own.
func (p *Plugin) Add(name string) *Player {
	pl := &Player{ID: control.PlayerID(len(p.players) + 1), Name: name, Camera: p.worldPlugin.Camera(), View: p.worldPlugin.View(), world: p.worldPlugin}
	p.players = append(p.players, pl)
	return pl
}

// Players lists every player, in the order added.
func (p *Plugin) Players() []*Player { return p.players }

// Locals lists the players at this keyboard.
func (p *Plugin) Locals() []*Player {
	var out []*Player
	for _, pl := range p.players {
		if pl.local {
			out = append(out, pl)
		}
	}
	return out
}

// ByID is the player with id, or nil — for Nobody too.
func (p *Plugin) ByID(id control.PlayerID) *Player {
	if id == control.Nobody || int(id) > len(p.players) {
		return nil
	}
	return p.players[id-1]
}

// Defaults is every handler's default bindings, camera controls included, in one list to bind.
func (p *Plugin) Defaults() []control.Binding {
	var out []control.Binding
	for _, c := range p.handlers {
		out = append(out, c.DefaultBindings()...)
	}
	return out
}

// Issue gives cmd as player (nil for Nobody); ErrUnknownCommand when no handler defines its
// type. Bindings, an AI or a network all come in here.
func (p *Plugin) Issue(player *Player, cmd any) error {
	box, ok := p.queues[reflect.TypeOf(cmd)]
	if !ok {
		return fmt.Errorf("%w: %T", ErrUnknownCommand, cmd)
	}
	id := control.Nobody
	if player != nil {
		id = player.ID
	}
	box.Put(id, cmd)
	return nil
}

// =================================================================
// plugin.CommandHandler contract — players' own commands are Pan and Zoom
// =================================================================

func (p *Plugin) Queues() []control.CommandQueue { return []control.CommandQueue{&p.pans, &p.zooms} }

// DefaultBindings is CameraBindings at DefaultScrollSpeed.
func (p *Plugin) DefaultBindings() []control.Binding { return CameraBindings() }

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.players" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan carries out the camera commands and empties every queue; call it last in Update.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op — players draw nothing; a selection box is selection's to draw.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is nil — players draw nothing.
func (p *Plugin) Renderer() render.Layer { return nil }

// Layout splits the screen into n parts, one per camera the local players look through, in the
// order the players were added.
type Layout func(screen geom.AABB, n int) []geom.AABB

// Columns is the default Layout: n equal columns side by side, each a whole number of pixels wide,
// the last taking what is left over.
func Columns(screen geom.AABB, n int) []geom.AABB {
	out := make([]geom.AABB, n)
	w := math.Floor((screen.BottomRight.X - screen.TopLeft.X) / float64(n))
	for i := range out {
		x := screen.TopLeft.X + float64(i)*w
		out[i] = geom.NewAABB(geom.NewVec(x, screen.TopLeft.Y), geom.NewVec(x+w, screen.BottomRight.Y))
	}
	out[n-1].BottomRight.X = screen.BottomRight.X
	return out
}

// WithLayout sets how Viewports splits the screen between the local players' cameras.
func (p *Plugin) WithLayout(layout Layout) *Plugin {
	p.layout = layout
	return p
}

// Viewports are where the local players look: one per camera they look through, laid out by the
// Layout (Columns unless WithLayout), the world's camera over the whole screen when nobody is at
// this keyboard. A Scene showing the world hands them to the engine as its game.Viewer; each local
// player keeps its part of the screen, where its mouse input comes from.
func (p *Plugin) Viewports(screen geom.AABB) []render.Viewport {
	var cams []camera.Camera
	for _, pl := range p.Locals() {
		if !slices.Contains(cams, pl.Camera) {
			cams = append(cams, pl.Camera)
		}
	}
	if len(cams) == 0 {
		return render.Whole(p.worldPlugin.Camera(), screen)
	}
	layout := p.layout
	if layout == nil {
		layout = Columns
	}
	areas := layout(screen, len(cams))
	out := make([]render.Viewport, len(cams))
	for i, cam := range cams {
		out[i] = render.Viewport{Camera: cam, Area: areas[i]}
	}
	for _, pl := range p.Locals() {
		pl.area = areas[slices.Index(cams, pl.Camera)]
	}
	return out
}

// EventHandler translates this tick's input through every local player's bindings; call it from
// the active Scene's HandleEvents.
func (p *Plugin) EventHandler() control.EventHandler { return eventHandler{p} }

// Serializable is the players' own cameras, in the order the players were added; the others are
// the world's, saved with it. Nil when nobody has a camera of their own.
func (p *Plugin) Serializable() plugin.Serializable {
	for _, pl := range p.players {
		if pl.own {
			return ownCameras{p}
		}
	}
	return nil
}

// Restore rebuilds the own cameras after a load has written their state.
func (p *Plugin) Restore() {
	for _, pl := range p.players {
		if pl.own {
			pl.Camera.Restore()
		}
	}
}

// ownCameras persists the players' own cameras.
type ownCameras struct{ p *Plugin }

func (o ownCameras) Persisted() []any {
	var out []any
	for _, pl := range o.p.players {
		if pl.own {
			out = append(out, pl.Camera.Persisted()...)
		}
	}
	return out
}

// RegisterBehavior reports ErrUnhostedBehavior — players host no behaviors; they carry commands.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		return fmt.Errorf("%w: %T in %s", plugin.ErrUnhostedBehavior, b, p.Name())
	}
	return nil
}
