package players

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

// Pan moves the player's camera by screen pixels, the same at any zoom.
type Pan struct{ Dx, Dy float32 }

// Zoom scales the player's camera about the world point At: a Factor above 1 zooms in, below 1 out.
type Zoom struct {
	Factor float32
	At     geom.Vec
}

// Give is the command by which an entity becomes To's — its alone, whoever's it was; Nobody's for
// control.Nobody. An entity gives it itself: as it is made (kind.Entry.Told), or in a rule or a
// plan — captured, converted. Its units take commands from that player alone from then on.
type Give struct{ To control.PlayerID }

// Follow is the command to fasten the player's camera over the one unit it has chosen — kept
// in the middle of the screen as it goes (camera.Centred) — or, fastened already, to let go: C by
// default, in a game with a Chooser. Given by an entity for itself (kind.Entry.Told, after its
// Give), its owner's camera is fastened over it from its first step.
type Follow struct{}

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

// Chooser is a handler that knows the one unit a player has chosen to be followed — the selection,
// whose Chosen is the one Selected unit the player owns; false with none, or several.
type Chooser interface {
	Chosen(by control.PlayerID) (uid.UID64, bool)
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
	return []control.CommandQueue{&p.pans, &p.zooms, &p.gives, &p.follows, &p.hands.drives, &p.quits, &p.listings, &p.saves}
}

// DefaultBindings is CameraBindings at DefaultScrollSpeed, GameBindings, the keys driving the
// unit the camera is fastened to (W, S, A and D riding inside it, the arrows behind it), in a
// game with a selection C to follow the selected unit, and, in a game that said where it saves
// (WithSaves), F5 to save.
func (p *Plugin) DefaultBindings() []control.Binding {
	keys := append(CameraBindings(), GameBindings()...)
	keys = append(keys, ridingBindings()...)
	if p.chooser != nil {
		keys = append(keys, control.Command(control.KeyPress{Key: control.KeyC}, "Follow the selected unit, or stop", func(control.Context) (Follow, bool) { return Follow{}, true }))
	}
	if p.savePath != "" {
		keys = append(keys, control.Command(control.KeyPress{Key: control.KeyF5}, "Save the game", func(control.Context) (Save, bool) { return Save{}, true }))
	}
	return keys
}

// GameBindings are the keys every game has: K for the list of shortcuts, Shift+Esc to quit. A
// game that binds its own camera keys in place of CameraBindings binds these beside them.
func GameBindings() []control.Binding {
	return []control.Binding{
		control.Command(control.KeyPress{Key: control.KeyK}, "Shortcuts; Esc closes them", func(control.Context) (ShowShortcuts, bool) { return ShowShortcuts{}, true }),
		control.Command(control.KeyPress{Key: control.KeyEscape, Mods: control.Mods{Shift: true}}, "Quit", func(control.Context) (Quit, bool) { return Quit{}, true }),
	}
}
