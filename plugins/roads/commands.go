package roads

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Aim is where the cursor of a player holding the road points: where its last leg runs to.
type Aim struct{ At geom.Vec }

// Mark lays a point of the road's route at the world point At: the first where the road
// begins, every next one it runs to, the last again lifts it; from the second on the road is
// planned whole and asked about.
type Mark struct{ At geom.Vec }

// Piece lays one piece of road in the cell under At toward the cell under Toward, joining the
// roads ending at the cell; Toward in the same cell joins them alone.
type Piece struct{ At, Toward geom.Vec }

// Build builds the road its player was asked about.
type Build struct{}

// Cancel builds nothing of the road its player was asked about, its route still marked.
type Cancel struct{}

// Refuse is a rule's answer to Estimated: the road planned may not be built, Why shown to its
// player.
type Refuse struct{ Why string }

// Levelling sets whether the tool levels the ground under the roads it lays
// (Settings.Levelling).
type Levelling struct{ On bool }

func (Aim) Command()       {}
func (Mark) Command()      {}
func (Piece) Command()     {}
func (Build) Command()     {}
func (Cancel) Command()    {}
func (Refuse) Command()    {}
func (Levelling) Command() {}

var (
	_ command.Command = Aim{}
	_ command.Command = Mark{}
	_ command.Command = Piece{}
	_ command.Command = Build{}
	_ command.Command = Cancel{}
	_ command.Command = Refuse{}
	_ command.Command = Levelling{}
)

// Queues are the plugin's command queues: Aim, Mark, Piece, Build, Cancel, Refuse and Levelling.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.aims, &p.marks, &p.pieces, &p.builds, &p.cancels,
		&p.refusals, &p.levels}
}

// DefaultBindings are Shift+R taking the road in hand and, while it is held, the cursor aiming
// it, a left click marking a point of its route, a left drag with Shift laying a piece toward a
// neighbour, and a right click or Esc putting it down.
func (p *Plugin) DefaultBindings() []control.Binding {
	aim := func(c control.Context) (Aim, bool) { return Aim{At: c.World(c.Cursor)}, true }
	mark := func(c control.Context) (Mark, bool) { return Mark{At: c.World(c.Cursor)}, true }
	piece := func(c control.Context) (Piece, bool) {
		return Piece{At: c.World(c.Start), Toward: c.World(c.Cursor)}, true
	}
	left, right := control.MouseButtonLeft, control.MouseButtonRight
	shift := control.Mods{Shift: true}
	return []control.Binding{
		control.Give(control.KeyPress{Key: control.KeyR, Mods: shift}, "Build a road",
			control.Take{Tool: Tool}),
		control.Build(control.CursorOver{}, "Lay the road out", aim).With(Tool),
		control.Build(control.ButtonHeld{Button: left}, "Lay the road out", aim).With(Tool),
		control.Build(control.ButtonHeld{Button: right}, "Lay the road out", aim).With(Tool),
		control.Build(control.Drag{Button: left}, "A point of the road here", mark).With(Tool),
		control.Build(control.Drag{Button: left, Mods: shift}, "A piece of road toward here",
			piece).With(Tool),
		control.Give(control.Drag{Button: right}, "Put the road down", control.Drop{}).
			With(Tool),
		control.Give(control.Drag{Button: right, Mods: shift}, "Put the road down",
			control.Drop{}).With(Tool),
		control.Give(control.KeyPress{Key: control.KeyEscape}, "Put the road down",
			control.Drop{}).With(Tool),
	}
}
