package underground

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Tool is a tunnel in a player's hand, for control.Take.
const Tool control.Tool = "tunnel"

// Aim is where the cursor of a player holding the tunnel points: where the tunnel runs to.
type Aim struct{ At geom.Vec }

// Mark lays a tunnel's head at the world point At: the first where the tunnel begins, the next
// its other end, asking whether to dig it; the first again lifts it.
type Mark struct{ At geom.Vec }

// Dig digs the tunnel its player was asked about.
type Dig struct{}

// Cancel digs nothing of the tunnel its player was asked about, the first head still laid.
type Cancel struct{}

// Courses has the courses of what lies under the ground and the units in it marked over the
// world, or not (Settings.HideCourses). The settings screen alone, under Game.
type Courses struct{ Shown bool }

func (Aim) Command()     {}
func (Mark) Command()    {}
func (Dig) Command()     {}
func (Cancel) Command()  {}
func (Courses) Command() {}

var (
	_ command.Command = Aim{}
	_ command.Command = Mark{}
	_ command.Command = Dig{}
	_ command.Command = Cancel{}
	_ command.Command = Courses{}
)

// Queues are the plugin's command queues: Aim, Mark, Dig, Cancel and Courses.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.aims, &p.marks, &p.digs, &p.cancels, &p.courses}
}

// Settings is whether what lies under the ground is marked over the world, under Game
// (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	return []control.Setting{control.Switch{
		Label:   "What lies under the ground is marked",
		Section: control.Game,
		Get:     func() bool { return !p.Res.Settings.HideCourses },
		Set:     func(on bool) command.Command { return Courses{Shown: on} },
	}}
}

// DefaultBindings are Shift+T taking the tunnel in hand and, while it is held, the cursor aiming
// it, a left click marking a head and a right click or Esc putting it down.
func (p *Plugin) DefaultBindings() []control.Binding {
	aim := func(c control.Context) (Aim, bool) { return Aim{At: c.World(c.Cursor)}, true }
	mark := func(c control.Context) (Mark, bool) { return Mark{At: c.World(c.Cursor)}, true }
	left, right := control.MouseButtonLeft, control.MouseButtonRight
	shift := control.Mods{Shift: true}
	return []control.Binding{
		control.Give(control.KeyPress{Key: control.KeyT, Mods: shift}, "Dig a tunnel",
			control.Take{Tool: Tool}),
		control.Build(control.CursorOver{}, "Lay the tunnel out", aim).With(Tool),
		control.Build(control.ButtonHeld{Button: left}, "Lay the tunnel out", aim).With(Tool),
		control.Build(control.ButtonHeld{Button: right}, "Lay the tunnel out", aim).With(Tool),
		control.Build(control.Drag{Button: left}, "A tunnel's head here", mark).With(Tool),
		control.Build(control.Drag{Button: left, Mods: shift}, "A tunnel's head here", mark).
			With(Tool),
		control.Give(control.Drag{Button: right}, "Put the tunnel down", control.Drop{}).
			With(Tool),
		control.Give(control.Drag{Button: right, Mods: shift}, "Put the tunnel down",
			control.Drop{}).With(Tool),
		control.Give(control.KeyPress{Key: control.KeyEscape}, "Put the tunnel down",
			control.Drop{}).With(Tool),
	}
}
