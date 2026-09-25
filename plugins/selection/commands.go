package selection

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/uid"
)

// Select is the command to select: exactly IDs when given, else every Selectable entity in Box
// (world units); Additive keeps what was selected before.
type Select struct {
	IDs []uid.UID64
	Box geom.AABB
	// Screen is the screen rectangle the player picked in; set, it decides which of the entities
	// in Box are hit by where they are drawn — a unit standing high or flying is where it is seen.
	Screen   geom.AABB
	Additive bool
}

// Follow is the command to follow the one selected unit with the camera, or to stop following.
type Follow struct{}

var _ plugin.Commander = (*Plugin)(nil)

// Commands is the inboxes Select and Follow land in — for the players plugin.
func (p *Plugin) Commands() []control.Mailbox { return []control.Mailbox{&p.selects, &p.follows} }

// DefaultBindings is a left drag (a click is a drag of no length) into a Select of the box it
// drew, Shift for an additive one, and F to follow the one selected unit or stop following.
func (p *Plugin) DefaultBindings() []control.Binding {
	box := func(additive bool) func(c control.Context) (Select, bool) {
		return func(c control.Context) (Select, bool) {
			return Select{Box: c.WorldBox(c.Start, c.Cursor), Screen: control.ScreenRect(c.Start, c.Cursor), Additive: additive}, true
		}
	}
	return []control.Binding{
		control.Command(control.Drag{Button: ebiten.MouseButtonLeft}, "Select", box(false)),
		control.Command(control.Drag{Button: ebiten.MouseButtonLeft, Mods: control.Mods{Shift: true}}, "Add to selection", box(true)),
		control.Command(control.KeyPress{Key: ebiten.KeyF}, "Follow the selected unit", func(control.Context) (Follow, bool) {
			return Follow{}, true
		}),
	}
}
