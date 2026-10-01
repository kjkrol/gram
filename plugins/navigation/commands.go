package navigation

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

// MoveTo is the command to send every Selected entity to Cell, or with Append to add Cell behind
// the orders they already have; given by an entity for itself, it sends that entity alone. At is the world point clicked: under BodySpacing where the group
// stands round, under CellSpacing where one standing on Cell turns instead of going anywhere.
type MoveTo struct {
	Cell   cell.ID
	At     geom.Vec
	Append bool
}

// LookAt is the command to every Selected entity — or to the entity that gives it itself — to
// finish the step it is on, stop and turn towards At.
type LookAt struct {
	At geom.Vec
}

// The commands a unit gives itself among others (Order in a rule or a plan), carried out for
// that unit alone; those that name whom they are about are told it as they are given (rule.Aimed).
type (
	// StepAside has a unit standing step off the way of Of, beside, where the ground takes it and
	// nobody stands — never into water or a hole, off a cliff or onto a step (yieldClimb) — and stay
	// there; with nowhere to, it stands. On the move it steps aside a while, then goes on to its
	// own goals.
	StepAside struct{ Of uid.UID64 }
	// Detour has a unit on the move go round Of: it steps round them a while and, when they stand,
	// notes their cell for its routes to go round and plans afresh; Touch and Blocked say it is
	// cornered with no way round.
	Detour struct{ Of uid.UID64 }
	// Pass has a unit on the move go on past Of, who makes way for it: under BodySpacing it steps
	// round them a while, its route kept; under CellSpacing its step waits for the cell.
	Pass struct{ Of uid.UID64 }
	// Hold keeps a unit on the move where it stands till the way ahead clears, stallAfter at most;
	// then Touch and Blocked say it waited out.
	Hold struct{}
	// Settle has a unit on the move stand beside its goal, Beside on it.
	Settle struct{ Beside uid.UID64 }
	// Stop ends a unit's order where it stands, as come to the end of it.
	Stop struct{}
)

// Aim tells the command whose way it is.
func (s *StepAside) Aim(who uid.UID64) { s.Of = who }

// Aim tells the command whom to go round.
func (d *Detour) Aim(who uid.UID64) { d.Of = who }

// Aim tells the command whom to go past.
func (p *Pass) Aim(who uid.UID64) { p.Of = who }

// Aim tells the command who stands on the goal.
func (s *Settle) Aim(who uid.UID64) { s.Beside = who }

// Routes shows the routes of the selected units drawn, or hides them: their goals are drawn always.
// A look at the world, like the camera's turn: at once, in the pause too, not saved.
type Routes struct{}

// clickSlop is how far, in pixels, the cursor may move between a button going down and coming up
// for the release to be a click and not a drag.
const clickSlop = 4

// dragged reports whether the cursor moved further than clickSlop since the button went down.
func dragged(c control.Context) bool {
	return math.Hypot(c.Cursor.X-c.Start.X, c.Cursor.Y-c.Start.Y) > clickSlop
}

var _ plugin.CommandHandler = (*Plugin)(nil)

// Queues are where MoveTo, LookAt and Routes land — for the players plugin — and the commands a
// unit gives itself among others: StepAside, Detour, Pass, Hold, Settle and Stop.
func (p *Plugin) Queues() []control.CommandQueue {
	return append([]control.CommandQueue{&p.moves, &p.looks, &p.routes}, p.given.all()...)
}

// DefaultBindings is a right click — the button up where it went down, within clickSlop — into a
// MoveTo of the cell under the cursor, Shift to append; a right drag into a LookAt of the point
// under the cursor at every move, so the selected units turn to look where the cursor goes, and
// nothing on its release; and Shift+P into Routes.
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
		control.Command(control.Drag{Button: control.MouseButtonRight}, "Move selected units here", to(false)),
		control.Command(control.Drag{Button: control.MouseButtonRight, Mods: control.Mods{Shift: true}}, "Add a waypoint", to(true)),
		control.Command(control.ButtonHeld{Button: control.MouseButtonRight}, "Turn selected units to look at the cursor", turn),
		control.Command(control.KeyPress{Key: control.KeyP, Mods: control.Mods{Shift: true}}, "Show or hide the routes of selected units", func(control.Context) (Routes, bool) { return Routes{}, true }),
	}
}
