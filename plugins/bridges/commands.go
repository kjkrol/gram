package bridges

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Aim is where the cursor of a player holding the bridge points: where the bridge runs to.
type Aim struct{ At geom.Vec }

// Mark lays a bridge's head at the world point At: the first where the bridge begins, the next
// its other end, asking whether to build it; the first again lifts it.
type Mark struct{ At geom.Vec }

// Build builds the bridge its player was asked about.
type Build struct{}

// Cancel builds nothing of the bridge its player was asked about, the first head still laid.
type Cancel struct{}

func (Aim) Command()    {}
func (Mark) Command()   {}
func (Build) Command()  {}
func (Cancel) Command() {}

var (
	_ command.Command = Aim{}
	_ command.Command = Mark{}
	_ command.Command = Build{}
	_ command.Command = Cancel{}
)

// Queues are the plugin's command queues: Aim, Mark, Build and Cancel.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.aims, &p.marks, &p.builds, &p.cancels}
}

// DefaultBindings are Shift+B taking the bridge in hand and, while it is held, the cursor aiming
// it, a left click marking a head and a right click or Esc putting it down.
func (p *Plugin) DefaultBindings() []control.Binding {
	aim := func(c control.Context) (Aim, bool) { return Aim{At: c.World(c.Cursor)}, true }
	mark := func(c control.Context) (Mark, bool) { return Mark{At: c.World(c.Cursor)}, true }
	left, right := control.MouseButtonLeft, control.MouseButtonRight
	shift := control.Mods{Shift: true}
	return []control.Binding{
		control.Give(control.KeyPress{Key: control.KeyB, Mods: shift}, "Build a bridge",
			control.Take{Tool: Tool}),
		control.Build(control.CursorOver{}, "Lay the bridge out", aim).With(Tool),
		control.Build(control.ButtonHeld{Button: left}, "Lay the bridge out", aim).With(Tool),
		control.Build(control.ButtonHeld{Button: right}, "Lay the bridge out", aim).With(Tool),
		control.Build(control.Drag{Button: left}, "A bridge's head here", mark).With(Tool),
		control.Build(control.Drag{Button: left, Mods: shift}, "A bridge's head here", mark).
			With(Tool),
		control.Give(control.Drag{Button: right}, "Put the bridge down", control.Drop{}).
			With(Tool),
		control.Give(control.Drag{Button: right, Mods: shift}, "Put the bridge down",
			control.Drop{}).With(Tool),
		control.Give(control.KeyPress{Key: control.KeyEscape}, "Put the bridge down",
			control.Drop{}).With(Tool),
	}
}
