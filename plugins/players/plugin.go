package players

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// ErrUnknownCommand is what Issue reports for a command type none of the Commanders defines.
var ErrUnknownCommand = errors.New("players: no plugin listens for this command")

// Plugin keeps the game's players and carries their commands to the Commanders that define them:
// a player's bindings, an AI or a network issue a command, and it lands in its owner's inbox.
type Plugin struct {
	worldPlugin *world.Plugin
	ground      func(x, y float32) float32 // the world's Ground for command contexts, bound at first use
	commanders  []plugin.Commander
	players     []*Player
	inboxes     map[reflect.Type]control.Mailbox
	pans        control.Inbox[Pan]
	zooms       control.Inbox[Zoom]
	module      *module
	renderer    *Renderer
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.Commander = (*Plugin)(nil)

// NewPlugin builds the players plugin over worldPlugin, whose camera and View the local players
// share, carrying the commands of commanders; two Commanders of one command type panic.
func NewPlugin(worldPlugin *world.Plugin, commanders ...plugin.Commander) *Plugin {
	p := &Plugin{worldPlugin: worldPlugin, inboxes: map[reflect.Type]control.Mailbox{}}
	p.commanders = append([]plugin.Commander{p}, commanders...)
	for _, c := range p.commanders {
		for _, box := range c.Commands() {
			if other, taken := p.inboxes[box.Accepts()]; taken && other != box {
				panic(fmt.Sprintf("players: %v is defined twice", box.Accepts()))
			}
			p.inboxes[box.Accepts()] = box
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
	pl := &Player{ID: control.PlayerID(len(p.players) + 1), Name: name, Camera: p.worldPlugin.Camera(), View: p.worldPlugin.View()}
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

// Defaults is every Commander's default bindings, camera controls included, in one list to bind.
func (p *Plugin) Defaults() []control.Binding {
	var out []control.Binding
	for _, c := range p.commanders {
		out = append(out, c.DefaultBindings()...)
	}
	return out
}

// Issue gives cmd as player (nil for Nobody); ErrUnknownCommand when no Commander defines its
// type. Bindings, an AI or a network all come in here.
func (p *Plugin) Issue(player *Player, cmd any) error {
	box, ok := p.inboxes[reflect.TypeOf(cmd)]
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
// plugin.Commander contract — players' own commands are Pan and Zoom
// =================================================================

func (p *Plugin) Commands() []control.Mailbox { return []control.Mailbox{&p.pans, &p.zooms} }

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

// RunPlan carries out the camera commands and empties every inbox; call it last in Update.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer builds the marquee renderer; atlas is unused, players draw primitives.
func (p *Plugin) WithRenderer(render.AtlasSource) { p.renderer = &Renderer{p: p} }

// Renderer returns the marquee renderer, or nil unless WithRenderer was called.
func (p *Plugin) Renderer() render.Renderer {
	if p.renderer == nil {
		return nil
	}
	return p.renderer
}

// Renderers is what the player's plugins draw over the world — each Commander's Renderer, in the
// order given to NewPlugin, then the marquee — for the Scene to lay over its world layers.
func (p *Plugin) Renderers() []render.Renderer {
	var out []render.Renderer
	for _, c := range p.commanders[1:] {
		if pl, ok := c.(plugin.Plugin); ok {
			if r := pl.Renderer(); r != nil {
				out = append(out, r)
			}
		}
	}
	if p.renderer != nil {
		out = append(out, p.renderer)
	}
	return out
}

// EventHandler translates this tick's input through every local player's bindings; call it from
// the active Scene's HandleEvents.
func (p *Plugin) EventHandler() control.EventHandler { return translator{p} }

// Serializable returns nil — the local players' cameras are the world's, saved with it.
func (p *Plugin) Serializable() plugin.Serializable { return nil }

// RegisterBehavior reports ErrUnhostedBehavior — players host no behaviors; they carry commands.
func (p *Plugin) RegisterBehavior(behaviors ...plugin.Behavior) error {
	for _, b := range behaviors {
		return fmt.Errorf("%w: %T in %s", plugin.ErrUnhostedBehavior, b, p.Name())
	}
	return nil
}
