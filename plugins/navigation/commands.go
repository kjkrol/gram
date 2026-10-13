package navigation

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

// MoveTo is the command to send every Selected entity to Cell, or with Append to add Cell behind
// the orders they already have; given by an entity for itself, it sends that entity alone. At is
// the world point clicked, where the group stands round or the way one standing on Cell turns.
// To names the goal in place of Cell: the one entity bearing the name, a cell or a unit.
type MoveTo struct {
	Cell   cell.ID
	To     ceremony.Whom
	At     geom.Vec
	Append bool
	Level  Level
}

// Level is which of a cell's two levels, a raised deck or the ground under it, a goal lies on. A
// cell of one level takes any.
type Level uint8

const (
	EitherLevel Level = iota
	OnGround
	OnDeck
)

// world units under the point picked within which a deck still counts as picked
const pickReach = 1e-2

// LookAt is the command to every Selected entity — or to the entity that gives it itself — to
// finish the step it is on, stop and turn towards At.
type LookAt struct {
	At geom.Vec
}

// The commands a unit gives itself among others, carried out for that unit alone.
type (
	// StepAside has a unit standing step off the way of Of where the ground takes it and nobody
	// stands, and stay there; on the move it steps aside a while, then goes on to its own goals.
	StepAside struct{ Of uid.UID64 }
	// Detour has a unit on the move go round Of and plan afresh once they stand.
	Detour struct{ Of uid.UID64 }
	// Pass has a unit on the move go on past Of, who makes way for it.
	Pass struct{ Of uid.UID64 }
	// Hold keeps a unit on the move where it stands till the way ahead clears, a while at most.
	Hold struct{}
	// Settle has a unit on the move stand beside its goal, Beside on it.
	Settle struct{ Beside uid.UID64 }
	// Stop ends a unit's order where it stands, as come to the end of it.
	Stop struct{}
)

var (
	_ host.Aimed = (*StepAside)(nil)
	_ host.Aimed = (*Detour)(nil)
	_ host.Aimed = (*Pass)(nil)
	_ host.Aimed = (*Settle)(nil)
)

// Aim tells the command whose way it is.
func (s *StepAside) Aim(who uid.UID64) { s.Of = who }

// Aim tells the command whom to go round.
func (d *Detour) Aim(who uid.UID64) { d.Of = who }

// Aim tells the command whom to go past.
func (p *Pass) Aim(who uid.UID64) { p.Of = who }

// Aim tells the command who stands on the goal.
func (s *Settle) Aim(who uid.UID64) { s.Beside = who }

// Routes is the command showing the routes of the selected units drawn, or hiding them; their
// goals are drawn always.
type Routes struct{}

func (MoveTo) Command()    {}
func (LookAt) Command()    {}
func (StepAside) Command() {}
func (Detour) Command()    {}
func (Pass) Command()      {}
func (Hold) Command()      {}
func (Settle) Command()    {}
func (Stop) Command()      {}
func (Routes) Command()    {}

var (
	_ command.Command = MoveTo{}
	_ command.Command = LookAt{}
	_ command.Command = StepAside{}
	_ command.Command = Detour{}
	_ command.Command = Pass{}
	_ command.Command = Hold{}
	_ command.Command = Settle{}
	_ command.Command = Stop{}
	_ command.Command = Routes{}
	_ command.Command = Onward{}
)

// pixels the cursor may move between the button going down and up for a click, not a drag
const clickSlop = 4

func dragged(c control.Context) bool {
	return math.Hypot(c.Cursor.X-c.Start.X, c.Cursor.Y-c.Start.Y) > clickSlop
}

var _ plugin.CommandHandler = (*Plugin)(nil)

// Queues are the queues of every command the plugin carries out, for the players plugin.
func (p *Plugin) Queues() []command.CommandQueue {
	queues := []command.CommandQueue{&p.moves, &p.looks, &p.routes, &p.onwards}
	return append(queues, p.given.all()...)
}

// DefaultBindings is a right click into a MoveTo of the cell and level under the cursor, Shift
// appending, a right drag into a LookAt at every move, and Shift+P into Routes.
func (p *Plugin) DefaultBindings() []control.Binding {
	brd := p.boardPlugin.Board()
	to := func(appendIt bool) func(c control.Context) (MoveTo, bool) {
		return func(c control.Context) (MoveTo, bool) {
			if dragged(c) {
				return MoveTo{}, false
			}
			at, z, picked := c.Point(c.Cursor)
			if !picked {
				cell, ok := brd.CellFor(at, math.Inf(1), 0)
				return MoveTo{Cell: cell, At: at, Append: appendIt}, ok
			}
			cell, deck, ok := brd.LevelFor(at, z, pickReach)
			level := OnGround
			if deck {
				level = OnDeck
			}
			return MoveTo{Cell: cell, At: at, Append: appendIt, Level: level}, ok
		}
	}
	turn := func(c control.Context) (LookAt, bool) { return LookAt{At: c.World(c.Cursor)}, dragged(c) }
	return []control.Binding{
		control.Build(control.Drag{Button: control.MouseButtonRight}, "Move selected units here", to(false)),
		control.Build(control.Drag{Button: control.MouseButtonRight, Mods: control.Mods{Shift: true}}, "Add a waypoint", to(true)),
		control.Build(control.ButtonHeld{Button: control.MouseButtonRight}, "Turn selected units to look at the cursor", turn),
		control.Build(control.KeyPress{Key: control.KeyP, Mods: control.Mods{Shift: true}}, "Show or hide the routes of selected units", func(control.Context) (Routes, bool) { return Routes{}, true }),
	}
}
