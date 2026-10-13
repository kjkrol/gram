package navigation

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/rule/cond"
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

// PushedByAlly is the word of a touch where Self stands and an ally on the move, not of its own
// group nor giving
// way, comes at it.
func PushedByAlly() cond.Word[Touch] { return cond.NewWord("pushedByAlly", Touch.pushedByAlly) }

func (t Touch) pushedByAlly() bool {
	return !t.Moving && t.OtherMoving && !t.OtherGivingWay && t.Ally && !t.Groupmate
}

// ReachedTheGroup is the word of a touch where Self, on the move to the last goal of its order,
// touches one of
// that order standing.
func ReachedTheGroup() cond.Word[Touch] {
	return cond.NewWord("reachedTheGroup", Touch.reachedTheGroup)
}

func (t Touch) reachedTheGroup() bool {
	return t.Moving && !t.GivingWay && t.LastGoal && !t.OtherMoving && t.Groupmate
}

// InTheWay is the word of a touch where Self is on the move and Other is in its way, not one of its
// order that
// has arrived.
func InTheWay() cond.Word[Touch] { return cond.NewWord("inTheWay", Touch.inTheWay) }

func (t Touch) inTheWay() bool { return t.Moving && !t.reachedTheGroup() }

// ClearsTheWay is the word of a touch where an idle ally in Self's way with room to make way for
// it.
func ClearsTheWay() cond.Word[Touch] { return cond.NewWord("clearsTheWay", Touch.clearsTheWay) }

func (t Touch) clearsTheWay() bool {
	return t.inTheWay() && !t.OtherMoving && t.Ally && !t.Groupmate && t.Room
}

// Blocks is the word of a touch where Other is in Self's way and does not make way for it.
func Blocks() cond.Word[Touch] { return cond.NewWord("blocks", Touch.blocks) }

func (t Touch) blocks() bool { return t.inTheWay() && !t.clearsTheWay() }

// GoalTaken is the word of a touch where one standing on Self's goal who does not make way for it.
func GoalTaken() cond.Word[Touch] { return cond.NewWord("goalTaken", Touch.goalTaken) }

func (t Touch) goalTaken() bool {
	return t.Moving && t.OnMyGoal && !t.OtherMoving && !t.clearsTheWay() && !t.reachedTheGroup()
}

// StrangerOnMyGoal is the word of a touch where a stranger standing on Self's goal.
func StrangerOnMyGoal() cond.Word[Touch] {
	return cond.NewWord("strangerOnMyGoal", Touch.strangerOnMyGoal)
}

func (t Touch) strangerOnMyGoal() bool { return !t.Ally && !t.OtherMoving && t.OnMyGoal }

// WaitsFirst is the word of a touch where Self is the one of two coming at each other that waits,
// the lower id,
// till its Hold runs out.
func WaitsFirst() cond.Word[Touch] { return cond.NewWord("waitsFirst", Touch.waitsFirst) }

func (t Touch) waitsFirst() bool { return t.HeadOn && t.Self < t.Other && !t.WaitedOut }

// NoWayRound is the word of a touch where Self's Detour found no way round the one in its way.
func NoWayRound() cond.Word[Touch] { return cond.NewWord("noWayRound", Touch.noWayRound) }

func (t Touch) noWayRound() bool { return t.Moving && t.Cornered }

// WaitedLong is the word of a touch where Self's Hold ran out with the way still closed.
func WaitedLong() cond.Word[Touch] { return cond.NewWord("waitedLong", Touch.waitedLong) }

func (t Touch) waitedLong() bool { return t.WaitedOut }
