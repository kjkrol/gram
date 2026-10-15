package players

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
)

// ErrUnknownCommand is what Issue reports for a command type no handler defines.
var ErrUnknownCommand = errors.New("players: no plugin listens for this command")

// Plugin keeps the game's players and gives their commands to the world's carrier, which takes
// each to the handler that defines it.
type Plugin struct {
	*world.Self

	worldPlugin *world.Plugin
	sections    section.Reader
	handlers    []plugin.CommandHandler
	origins     map[string]plugin.CommandHandler
	players     []*Player
	listings    command.Queue[ShowShortcuts]
	panels      command.Queue[telemetry.Panel]
	shortcuts   *Shortcuts
	now         func() time.Time // the clock KeyReleased measures by
	owned       *goke.Query
	ownedBase   goke.Comp[world.Base]
	ownedBy     goke.OptComp[tag.Tags[owner.Family]]
	gives       command.Queue[Give]
	takes       command.Queue[control.Take]
	drops       command.Queue[control.Drop]
	pass        uint64
	module      *module
	captured    bool
	setCapture  func(on bool)
}

var _ plugin.Plugin = (*Plugin)(nil)
var _ plugin.CommandHandler = (*Plugin)(nil)

// NewPlugin builds the players plugin over worldPlugin, having the world carry the commands of
// handlers beside its own and the players'. Two handlers of one command type panic.
func NewPlugin(worldPlugin *world.Plugin, handlers ...plugin.CommandHandler) *Plugin {
	for id := command.Issuer(1); int(id) <= owner.Players; id++ {
		if tag := worldPlugin.Kinds().DefineTag[owner.Family](owner.Name(id)); tag != owner.Of(id) {
			panic(fmt.Sprintf("players: the owners' family has tags of its own before %q", owner.Name(id)))
		}
	}
	worldPlugin.Roster().Unit.Default(comp.Marks[owner.Family]()) // every unit may be given
	p := &Plugin{worldPlugin: worldPlugin, now: time.Now}
	p.Self = world.NewSelf(worldPlugin, p)
	p.shortcuts = newShortcuts(p)
	p.handlers = append([]plugin.CommandHandler{p, worldPlugin}, handlers...)
	if err := worldPlugin.Carry(p.handlers...); err != nil {
		panic(fmt.Sprintf("players: %v", err))
	}
	return p
}

// Local adds a player at this keyboard, acting through the picture of the world a scene wires it
// to (Through); bind it before Use. Two local players acting through pictures of their own split
// the screen.
func (p *Plugin) Local(name string) *Player {
	pl := p.Add(name)
	pl.local = true
	return pl
}

// Add adds a player without a keyboard, an AI or a remote client, whose commands come in
// through Issue.
func (p *Plugin) Add(name string) *Player {
	if err := p.worldPlugin.InSection(fmt.Sprintf("player %q added", name), section.Players); err != nil {
		panic("players: " + err.Error())
	}
	pl := &Player{
		ID:       command.Issuer(len(p.players) + 1),
		Name:     name,
		world:    p.worldPlugin,
		sections: p.sections,
	}
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
func (p *Plugin) ByID(id command.Issuer) *Player {
	if id == command.Nobody || int(id) > len(p.players) {
		return nil
	}
	return p.players[id-1]
}

// Defaults is every handler's default bindings in one list to bind.
func (p *Plugin) Defaults() []control.Binding {
	var out []control.Binding
	for _, c := range p.handlers {
		for _, b := range c.DefaultBindings() {
			p.note(b, c)
			out = append(out, b)
		}
	}
	return out
}

// note remembers which handler's defaults a binding came from, for the list of shortcuts
func (p *Plugin) note(b control.Binding, h plugin.CommandHandler) {
	if p.origins == nil {
		p.origins = map[string]plugin.CommandHandler{}
	}
	p.origins[Written(b.Trigger)+" "+b.Label] = h
}

// the handler a binding came from, else the one carrying out its command
func (p *Plugin) originOf(b control.Binding) plugin.CommandHandler {
	if h, ok := p.origins[Written(b.Trigger)+" "+b.Label]; ok {
		return h
	}
	return p.handlerOf(b.Type())
}

// Issue gives cmd as player (nil for Nobody); ErrUnknownCommand when no handler defines its
// type.
func (p *Plugin) Issue(player *Player, cmd command.Command) error {
	id := command.Nobody
	if player != nil {
		id = player.ID
	}
	if !p.worldPlugin.Carrier().Put(id, cmd) {
		return fmt.Errorf("%w: %T", ErrUnknownCommand, cmd)
	}
	return nil
}

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

// Handle takes a scene's input for the tick: the players' bindings turn it into commands,
// ShowShortcuts is carried out at once and the game's own keys (OwnKeys) are run.
func (p *Plugin) Handle(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	p.EventHandler().HandleEvents(events)
	p.listings.Drain(func(command.Issued[ShowShortcuts]) { p.shortcuts.Open(runtime, composition) })
	p.panels.Drain(func(command.Issued[telemetry.Panel]) {
		if tp := telemetry.Of(p.worldPlugin); tp != nil {
			tp.Toggle(composition)
		}
	})
	p.shortcuts.keys.Handle(events, runtime, composition)
	p.pass++
}

func (p *Plugin) tools() {
	p.takes.Drain(func(i command.Issued[control.Take]) {
		if pl := p.ByID(i.Issuer); pl != nil {
			pl.tool = i.Command.Tool
		}
	})
	p.drops.Drain(func(i command.Issued[control.Drop]) {
		if pl := p.ByID(i.Issuer); pl != nil {
			pl.tool = ""
		}
	})
}

func (p *Plugin) Name() string { return "gram.players" }

func (p *Plugin) Install(ctx plugin.Installer) error {
	p.sections, _ = ctx.(section.Reader)
	p.module = &module{p: p}
	ctx.UseModule(p.module)
	return nil
}

// RunPlan hands over the units given and issues the keys held for the next tick; call it last in
// Update.
func (p *Plugin) RunPlan(ctx goke.RunCtx, d time.Duration) { p.module.RunPlan(ctx, d) }

// EventHandler translates this tick's input through every local player's bindings.
func (p *Plugin) EventHandler() control.EventHandler { return eventHandler{p} }

// Serializable is nil: players keep no state of their own.
func (p *Plugin) Serializable() plugin.Serializable { return nil }
