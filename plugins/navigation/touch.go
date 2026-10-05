package navigation

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

// Touch is one unit touching another, seen from Self, as navigation perceives it: under
// BodySpacing their boxes met, under CellSpacing Self was refused a step into the cell Other holds
// — or Other into Self's. It is the moment of the rules navigation hosts (rule.Then),
// handed every tick the two touch; the crowd's are the ready ones.
type Touch struct {
	Self, Other uid.UID64
	// Way is the way Self leaves Other, a unit vector.
	Way geom.Vec
	// Moving says Self is under an order, OtherMoving Other; GivingWay and OtherGivingWay that the
	// order is one to give way (StepAside).
	Moving, OtherMoving       bool
	GivingWay, OtherGivingWay bool
	// LastGoal says Self heads for the last goal of its order.
	LastGoal bool
	// Ally says the two are allies (players/owner.Allies); Groupmate that one MoveTo sent them —
	// the order each is under, or the last it came to the end of (LastOrder).
	Ally, Groupmate bool
	// OnMyGoal says Other stands on Self's goal; Room that Other, standing, has somewhere to step
	// off Self's way — ground that takes it, no steeper than yieldClimb, nobody there (StepAside).
	OnMyGoal, Room bool
	// HeadOn says the two, both on the move, come at each other.
	HeadOn bool
	// WaitedOut says Self's Hold ran out with the way still closed, Cornered that its Detour found
	// no way round — both till its next step.
	WaitedOut, Cornered bool
}

// Who is Self: whose moment it is.
func (t Touch) Who() uid.UID64 { return t.Self }

// Whom tells each the other one.
func (t Touch) Whom(each func(uid.UID64)) { each(t.Other) }

// Subject is the other one: whom an Aimed command given on the moment is about.
func (t Touch) Subject() (uid.UID64, bool) { return t.Other, true }

// PushedByAlly: Self stands, and an ally on the move comes at it — not one giving way itself, nor
// one of the order Self came to the end of, which stops beside it instead (ReachedTheGroup).
func (t Touch) PushedByAlly() bool {
	return !t.Moving && t.OtherMoving && !t.OtherGivingWay && t.Ally && !t.Groupmate
}

// ReachedTheGroup: Self, on the move to the last goal of its order, touches one of that order
// standing — come to the end of it before Self.
func (t Touch) ReachedTheGroup() bool {
	return t.Moving && !t.GivingWay && t.LastGoal && !t.OtherMoving && t.Groupmate
}

// InTheWay: Self is on the move and Other is in its way — anyone but one of its order that has
// come to the end of it.
func (t Touch) InTheWay() bool { return t.Moving && !t.ReachedTheGroup() }

// ClearsTheWay: an ally with nothing to do stands in Self's way, with room to make way for it
// (MakeWay).
func (t Touch) ClearsTheWay() bool {
	return t.InTheWay() && !t.OtherMoving && t.Ally && !t.Groupmate && t.Room
}

// Blocks: Other is in Self's way and does not make way for it — anyone but an idle ally with room.
func (t Touch) Blocks() bool { return t.InTheWay() && !t.ClearsTheWay() }

// GoalTaken: one standing on Self's goal who does not make way for it — a stranger, or an ally
// with no room to.
func (t Touch) GoalTaken() bool {
	return t.Moving && t.OnMyGoal && !t.OtherMoving && !t.ClearsTheWay() && !t.ReachedTheGroup()
}

// StrangerOnMyGoal: a stranger standing on Self's goal.
func (t Touch) StrangerOnMyGoal() bool { return !t.Ally && !t.OtherMoving && t.OnMyGoal }

// WaitsFirst: of two coming at each other, the one with the lower id, till its Hold runs out.
func (t Touch) WaitsFirst() bool { return t.HeadOn && t.Self < t.Other && !t.WaitedOut }

// NoWayRound: Self, on the move, found no way round the one in its way (Detour).
func (t Touch) NoWayRound() bool { return t.Moving && t.Cornered }

// WaitedLong: Self held till its Hold ran out, and the way is still closed.
func (t Touch) WaitedLong() bool { return t.WaitedOut }
