package navigation

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// Touch is the moment of one unit touching another, seen from Self: their boxes met, or Self was
// refused a step into the cell Other holds. It is handed to the rules every tick the two touch.
type Touch struct {
	Self, Other uid.UID64
	// Way is the way Self leaves Other, a unit vector.
	Way geom.Vec
	// Moving says Self is under an order, OtherMoving Other; GivingWay and OtherGivingWay that the
	// order is one to give way.
	Moving, OtherMoving       bool
	GivingWay, OtherGivingWay bool
	// LastGoal says Self heads for the last goal of its order.
	LastGoal bool
	// Ally says the two are allies; Groupmate that one MoveTo sent them.
	Ally, Groupmate bool
	// OnMyGoal says Other stands on Self's goal; Room that Other, standing, has somewhere to step
	// off Self's way.
	OnMyGoal, Room bool
	// HeadOn says the two, both on the move, come at each other.
	HeadOn bool
	// WaitedOut says Self's Hold ran out with the way still closed, Cornered that its Detour found
	// no way round.
	WaitedOut, Cornered bool
}

// Who is Self.
func (t Touch) Who() uid.UID64 { return t.Self }

// Whom tells each the other one.
func (t Touch) Whom(each func(uid.UID64)) { each(t.Other) }

// Subject is the other one, whom an Aimed command given on the moment is about.
func (t Touch) Subject() (uid.UID64, bool) { return t.Other, true }

// Touching is the condition that the other plays role, for a rule's When on a Touch.
func Touching(role *rule.Part) rule.Cond { return rule.Other(role.Tag()) }

// PushedByAlly reports that Self stands and an ally on the move, not of its own group nor giving
// way, comes at it.
func (t Touch) PushedByAlly() bool {
	return !t.Moving && t.OtherMoving && !t.OtherGivingWay && t.Ally && !t.Groupmate
}

// ReachedTheGroup reports that Self, on the move to the last goal of its order, touches one of
// that order standing.
func (t Touch) ReachedTheGroup() bool {
	return t.Moving && !t.GivingWay && t.LastGoal && !t.OtherMoving && t.Groupmate
}

// InTheWay reports that Self is on the move and Other is in its way, not one of its order that
// has arrived.
func (t Touch) InTheWay() bool { return t.Moving && !t.ReachedTheGroup() }

// ClearsTheWay reports an idle ally in Self's way with room to make way for it.
func (t Touch) ClearsTheWay() bool {
	return t.InTheWay() && !t.OtherMoving && t.Ally && !t.Groupmate && t.Room
}

// Blocks reports that Other is in Self's way and does not make way for it.
func (t Touch) Blocks() bool { return t.InTheWay() && !t.ClearsTheWay() }

// GoalTaken reports one standing on Self's goal who does not make way for it.
func (t Touch) GoalTaken() bool {
	return t.Moving && t.OnMyGoal && !t.OtherMoving && !t.ClearsTheWay() && !t.ReachedTheGroup()
}

// StrangerOnMyGoal reports a stranger standing on Self's goal.
func (t Touch) StrangerOnMyGoal() bool { return !t.Ally && !t.OtherMoving && t.OnMyGoal }

// WaitsFirst reports that Self is the one of two coming at each other that waits, the lower id,
// till its Hold runs out.
func (t Touch) WaitsFirst() bool { return t.HeadOn && t.Self < t.Other && !t.WaitedOut }

// NoWayRound reports that Self's Detour found no way round the one in its way.
func (t Touch) NoWayRound() bool { return t.Moving && t.Cornered }

// WaitedLong reports that Self's Hold ran out with the way still closed.
func (t Touch) WaitedLong() bool { return t.WaitedOut }
