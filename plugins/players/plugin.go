package players

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// ErrUnknownCommand is what Issue reports for a command type no handler defines.
var ErrUnknownCommand = errors.New("players: no plugin listens for this command")

// Plugin keeps the game's players and gives their commands to the world's carrier, which takes
// each to the handler that defines it: a player's bindings, an AI or a network issue a command,
// and it lands in its handler's queue.
type Plugin struct {
	*world.Self // its own entity: its knobs, the roles it plays, the effects it is under

	worldPlugin *world.Plugin
	handlers    []plugin.CommandHandler
	players     []*Player
	quits       control.Queue[Quit]
	listings    control.Queue[ShowShortcuts]
	saves       control.Queue[Save]
	shortcuts   *Shortcuts  // the scene listing the keys
	owned       *goke.Query // whose every entity is: what Through's Shows reads
	ownedBase   goke.Comp[world.Base]
	ownedBy     goke.OptComp[tag.Tags[owner.Family]]
	savePath    string // where Save writes; none, no saving
	saveWith    []any  // the game's own resources saved beside the plugins'
	gives       control.Queue[Give]
	module      *module
	// captured is whether the cursor is caught, as setCapture last set it; setCapture catches or
	// lets go of the window's cursor
	captured   bool
	setCapture func(on bool)
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin builds the players plugin over worldPlugin, having the world carry the commands of handlers — beside the world's own and the
// players' — as it does those of the plugins a stage uses; two handlers of one command type
// panic. It registers the owners' tags with the world's kinds, a
// player each, saved by name.
func NewPlugin(worldPlugin *world.Plugin, handlers ...plugin.CommandHandler) *Plugin {
	for id := control.PlayerID(1); int(id) <= owner.Players; id++ {
		if tag := worldPlugin.Kinds().DefineTag[owner.Family](owner.Name(id)); tag != owner.Of(id) {
			panic(fmt.Sprintf("players: the owners' family has tags of its own before %q", owner.Name(id)))
		}
	}
	worldPlugin.Roster().Unit.Default(comp.Marks[owner.Family]()) // every unit may be given
	p := &Plugin{Self: world.NewSelf(worldPlugin, "gram.players"), worldPlugin: worldPlugin}
	p.shortcuts = newShortcuts(p)
	p.handlers = append([]plugin.CommandHandler{p, worldPlugin}, handlers...)
	if err := worldPlugin.Carry(p.handlers...); err != nil {
		panic(fmt.Sprintf("players: %v", err))
	}
	return p
}

// Local adds a player at this keyboard, looking through cam (plugins/cameras); bind it before Use.
// Two local players looking through two cameras split the screen.
func (p *Plugin) Local(name string, cam camera.Camera) *Player {
	if cam == nil {
		panic(fmt.Sprintf("players: the local player %q needs a camera to look through", name))
	}
	pl := p.Add(name)
	pl.local = true
	pl.Camera, pl.View = cam, p.worldPlugin.ViewFor(cam)
	return pl
}

// Add adds a player without a keyboard — an AI, a remote client — whose commands come in through
// Issue; it looks through no camera.
func (p *Plugin) Add(name string) *Player {
	if err := p.worldPlugin.InSection(fmt.Sprintf("player %q added", name), section.Players); err != nil {
		panic("players: " + err.Error())
	}
	pl := &Player{ID: control.PlayerID(len(p.players) + 1), Name: name, world: p.worldPlugin}
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

// Defaults is every handler's default bindings in one list to bind: the cameras' among them when
// the cameras plugin is a handler.
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
	id := control.Nobody
	if player != nil {
		id = player.ID
	}
	if !p.worldPlugin.Carrier().Put(id, cmd) {
		return fmt.Errorf("%w: %T", ErrUnknownCommand, cmd)
	}
	return nil
}

// handlerOf is the handler whose queue takes commands of type t, nil for none of the players'.
func (p *Plugin) handlerOf(t reflect.Type) plugin.CommandHandler {
	for _, h := range p.handlers {
		for _, q := range h.Queues() {
			if q.Accepts() == t {
				return h
			}
		}
	}
	return nil
}

// =================================================================
// what a scene hands over
// =================================================================

// Handle takes a scene's input for the tick: the players' bindings turn it into commands, Quit
// and ShowShortcuts — which need the engine — are carried out at once, and the game's own keys
// (OwnKeys) are run. A scene showing the world calls it from its
// HandleEvents, and nothing else.
func (p *Plugin) Handle(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	p.EventHandler().HandleEvents(events)
	p.quits.Drain(func(control.Issued[Quit]) { runtime.Quit() })
	p.listings.Drain(func(control.Issued[ShowShortcuts]) { p.shortcuts.Open(runtime, composition) })
	p.saves.Drain(func(control.Issued[Save]) {
		if p.savePath == "" {
			return
		}
		if err := runtime.Persistence().Save(p.savePath, "", p.saveWith...); err != nil {
			log.Printf("players: save: %v", err)
			return
		}
		log.Printf("players: saved %q", p.savePath)
	})
	p.shortcuts.keys.Handle(events, runtime, composition)
}

// WithSaves says where the game is saved — basePath, with the game's own resources beside the
// plugins' — which gives the players the Save command its key, F5. Loading is the Stage's
// (Restore). Call it as the plugin is made.
func (p *Plugin) WithSaves(basePath string, resources ...any) *Plugin {
	p.savePath, p.saveWith = basePath, resources
	return p
}

// =================================================================
// plugin.Plugin contract
// =================================================================

func (p *Plugin) Name() string { return "gram.players" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan hands over the units given and issues the keys held for the next tick; call it last in
// Update.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// WithRenderer is a no-op — players draw nothing; a selection box is selection's to draw.
func (p *Plugin) WithRenderer(render.AtlasSource) {}

// Renderer is nil — players draw nothing.
func (p *Plugin) Renderer() render.Layer { return nil }

// EventHandler translates this tick's input through every local player's bindings; call it from
// the active Scene's HandleEvents.
func (p *Plugin) EventHandler() control.EventHandler { return eventHandler{p} }

// Serializable is nil — the players' cameras are the cameras plugin's to save.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
