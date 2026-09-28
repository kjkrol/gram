package navigation

import (
	"math"

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

// clickSlop is how far, in pixels, the cursor may move between a button going down and coming up
// for the release to be a click and not a drag.
const clickSlop = 4

// dragged reports whether the cursor moved further than clickSlop since the button went down.
func dragged(c control.Context) bool {
	return math.Hypot(c.Cursor.X-c.Start.X, c.Cursor.Y-c.Start.Y) > clickSlop
}

var _ plugin.CommandHandler = (*Plugin)(nil)

// Queues are where MoveTo and LookAt land — for the players plugin.
func (p *Plugin) Queues() []control.CommandQueue { return []control.CommandQueue{&p.moves, &p.looks} }

// DefaultBindings is a right click — the button up where it went down, within clickSlop — into a
// MoveTo of the cell under the cursor, Shift to append; and a right drag into a LookAt of the
// point under the cursor at every move, so the selected units turn to look where the cursor
// goes, and nothing on its release.
func (p *Plugin) DefaultBindings() []control.Binding {
	grid := p.boardPlugin.Res.Logic.Board.Grid
	to := func(appendIt bool) func(c control.Context) (MoveTo, bool) {
		return func(c control.Context) (MoveTo, bool) {
			if dragged(c) {
				return MoveTo{}, false // the drag turned the units; nothing moves
			}
			at := c.World(c.Cursor)
			cell, ok := grid.CellAt(at)
			return MoveTo{Cell: cell, At: at, Append: appendIt}, ok
		}
	}
	turn := func(c control.Context) (LookAt, bool) { return LookAt{At: c.World(c.Cursor)}, dragged(c) }
	return []control.Binding{
		control.Command(control.Drag{Button: ebiten.MouseButtonRight}, "Move selected units here", to(false)),
		control.Command(control.Drag{Button: ebiten.MouseButtonRight, Mods: control.Mods{Shift: true}}, "Add a waypoint", to(true)),
		control.Command(control.ButtonHeld{Button: ebiten.MouseButtonRight}, "Turn selected units to look at the cursor", turn),
	}
}
