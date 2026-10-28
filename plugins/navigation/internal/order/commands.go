package order

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
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

// March is the command to send every Selected entity the player owns, or the entity giving it,
// to Cell in a formation: as MoveTo sends them, the group laid out and paced by In.
type March struct {
	Cell  cell.ID
	To    ceremony.Whom
	At    geom.Vec
	Level Level
	In    Formation
}

// Level is which of a cell's two levels, a raised deck or the ground under it, a goal lies on. A
// cell of one level takes any.
type Level uint8

const (
	EitherLevel Level = iota
	OnGround
	OnDeck
)

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
	// Trail has a unit on the move follow Of, who goes its way: no faster than they go, its wait
	// no stall while they are under way.
	Trail struct{ Of uid.UID64 }
	// Stop ends a unit's order where it stands, as come to the end of it.
	Stop struct{}
)

var (
	_ host.Aimed = (*StepAside)(nil)
	_ host.Aimed = (*Detour)(nil)
	_ host.Aimed = (*Pass)(nil)
	_ host.Aimed = (*Settle)(nil)
	_ host.Aimed = (*Trail)(nil)
)

// Aim tells the command whose way it is.
func (s *StepAside) Aim(who uid.UID64) { s.Of = who }

// Aim tells the command whom to go round.
func (d *Detour) Aim(who uid.UID64) { d.Of = who }

// Aim tells the command whom to go past.
func (p *Pass) Aim(who uid.UID64) { p.Of = who }

// Aim tells the command who stands on the goal.
func (s *Settle) Aim(who uid.UID64) { s.Beside = who }

// Aim tells the command whom to follow.
func (t *Trail) Aim(who uid.UID64) { t.Of = who }

func (MoveTo) Command() {}

func (March) Command() {}

func (LookAt) Command() {}

func (StepAside) Command() {}

func (Detour) Command() {}

func (Pass) Command() {}

func (Hold) Command() {}

func (Settle) Command() {}

func (Trail) Command() {}

func (Stop) Command() {}

var (
	_ command.Command = MoveTo{}
	_ command.Command = March{}
	_ command.Command = LookAt{}
	_ command.Command = StepAside{}
	_ command.Command = Detour{}
	_ command.Command = Pass{}
	_ command.Command = Hold{}
	_ command.Command = Settle{}
	_ command.Command = Trail{}
	_ command.Command = Stop{}
	_ command.Command = Onward{}
)
