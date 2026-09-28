package navigation

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
)

// MoveTo is the command to send every Selected entity to Cell, or with Append to add Cell behind
// the orders they already have. At is the world point clicked: under BodySpacing where the group
// stands round, under CellSpacing where one standing on Cell turns instead of going anywhere.
type MoveTo struct {
	Cell   board.CellID
	At     geom.Vec
	Append bool
}

// LookAt is the command to every Selected entity to finish the step it is on, stop and turn
// towards At.
type LookAt struct {
	At geom.Vec
}

var _ plugin.CommandHandler = (*Plugin)(nil)

// Queues are where MoveTo and LookAt land — for the players plugin.
func (p *Plugin) Queues() []control.CommandQueue { return []control.CommandQueue{&p.moves, &p.looks} }

// DefaultBindings is a right click into a MoveTo of the cell under the cursor, Shift to append,
// and a right click with Shift and S held into a LookAt of the point under the cursor.
func (p *Plugin) DefaultBindings() []control.Binding {
	grid := p.boardPlugin.Res.Logic.Board.Grid
	to := func(appendIt bool) func(c control.Context) (MoveTo, bool) {
		return func(c control.Context) (MoveTo, bool) {
			at := c.World(c.Cursor)
			cell, ok := grid.CellAt(at)
			return MoveTo{Cell: cell, At: at, Append: appendIt}, ok
		}
	}
	look := func(c control.Context) (LookAt, bool) { return LookAt{At: c.World(c.Cursor)}, true }
	return []control.Binding{
		control.Command(control.ButtonPress{Button: ebiten.MouseButtonRight}, "Move selected units here", to(false)),
		control.Command(control.ButtonPress{Button: ebiten.MouseButtonRight, Mods: control.Mods{Shift: true}}, "Add a waypoint", to(true)),
		control.Command(control.ButtonPress{Button: ebiten.MouseButtonRight, Mods: control.Mods{Shift: true}.Holding(ebiten.KeyS)}, "Look there", look),
	}
}
