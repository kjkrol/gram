package players

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
)

// Give is the command by which an entity becomes To's — its alone, whoever's it was; Nobody's for
// control.Nobody. An entity gives it itself: as it is made (kind.Entry.Told), or in a rule or a
// plan — captured, converted. Its units take commands from that player alone from then on.
type Give struct{ To control.PlayerID }

// Drive is a player's hand on its units this tick: Ahead 1 walks them on the way each faces,
// -1 brakes and backs them away; Turn -1 or 1 turns them; Way, when not zero, is the way to go
// instead, as the screen lies; Sprint urges them on. Given every tick a key is held
// (control.KeyHeld), several in one tick adding up into the player's [Hand], which the plugin that
// moves units reads: navigation drives the unit the player's camera is fastened to, else the
// units it has selected. Given by an entity for itself (rule.Order), its own hand.
type Drive struct {
	Ahead, Turn int8
	Way         geom.Vec
	Sprint      bool
}

// Quit ends the game.
type Quit struct{}

// ShowShortcuts opens the list of every key of the game (the Shortcuts scene), or closes it.
type ShowShortcuts struct{}

// Save writes the game where the players were told to (Plugin.WithSaves). F5 by default, in a game
// that said where.
type Save struct{}

// Queues are where the players' own commands land.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.gives, &p.hands.drives, &p.quits, &p.listings, &p.saves}
}

// DefaultBindings are GameBindings, the keys driving the unit the camera is fastened to (W, S, A
// and D riding inside it, the arrows behind it) and, in a game that said where it saves
// (WithSaves), F5 to save.
func (p *Plugin) DefaultBindings() []control.Binding {
	keys := append(GameBindings(), ridingBindings()...)
	if p.savePath != "" {
		keys = append(keys, control.Command(control.KeyPress{Key: control.KeyF5}, "Save the game", func(control.Context) (Save, bool) { return Save{}, true }))
	}
	return keys
}

// GameBindings are the keys every game has: K for the list of shortcuts, Shift+Esc to quit. A
// game that binds its own keys in place of the Defaults binds these beside them.
func GameBindings() []control.Binding {
	return []control.Binding{
		control.Command(control.KeyPress{Key: control.KeyK}, "Shortcuts; Esc closes them", func(control.Context) (ShowShortcuts, bool) { return ShowShortcuts{}, true }),
		control.Command(control.KeyPress{Key: control.KeyEscape, Mods: control.Mods{Shift: true}}, "Quit", func(control.Context) (Quit, bool) { return Quit{}, true }),
	}
}
