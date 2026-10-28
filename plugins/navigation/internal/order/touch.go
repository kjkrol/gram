package order

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/fact"
	"github.com/kjkrol/ceremony/host"
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
	// Ahead says Other lies ahead of Self on its way: within a quarter turn of the way its route
	// leads it, or of its facing with none.
	Ahead bool
	// Along says Other moves Self's way.
	Along bool
	// SameWay says Other's route leads it Self's way too.
	SameWay bool
	// OtherTrailing says Other trails someone itself.
	OtherTrailing bool
	// NearMyGoal says Other stands within two cells of Self's goal.
	NearMyGoal bool
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
func PushedByAlly() cond.Word[Touch] { return cond.NewWord("pushed by ally", Touch.pushedByAlly) }

func (t Touch) pushedByAlly() bool {
	return !t.Moving && t.OtherMoving && !t.OtherGivingWay && t.Ally && !t.Groupmate
}

// ReachedTheGroup is the word of a touch where Self, on the move to the last goal of its order,
// touches one of that order standing near that goal.
func ReachedTheGroup() cond.Word[Touch] {
	return cond.NewWord("reached the group", Touch.reachedTheGroup)
}

// one of the order standing far from the goal gave up on the way: no group to join
func (t Touch) reachedTheGroup() bool {
	return t.Moving && !t.GivingWay && t.LastGoal && !t.OtherMoving && t.Groupmate && t.NearMyGoal
}

// InTheWay is the word of a touch where Self is on the move and Other is in its way: ahead of it,
// or not of its group — one of its group pressing on it from behind or beside is its queue, not
// its way.
func InTheWay() cond.Word[Touch] { return cond.NewWord("in the way", Touch.inTheWay) }

func (t Touch) inTheWay() bool {
	return t.Moving && (t.Ahead || !t.Groupmate) && !t.reachedTheGroup()
}

// ClearsTheWay is the word of a touch where an idle ally in Self's way with room to make way for
// it.
func ClearsTheWay() cond.Word[Touch] { return cond.NewWord("clears the way", Touch.clearsTheWay) }

func (t Touch) clearsTheWay() bool {
	return t.inTheWay() && !t.OtherMoving && t.Ally && !t.Groupmate && t.Room
}

// Blocks is the word of a touch where Other is in Self's way and does not make way for it.
func Blocks() cond.Word[Touch] { return cond.NewWord("blocks", Touch.blocks) }

func (t Touch) blocks() bool { return t.inTheWay() && !t.clearsTheWay() }

// GoalTaken is the word of a touch where one standing on Self's goal who does not make way for it.
func GoalTaken() cond.Word[Touch] { return cond.NewWord("goal taken", Touch.goalTaken) }

func (t Touch) goalTaken() bool {
	return t.Moving && t.OnMyGoal && !t.OtherMoving && !t.clearsTheWay() && !t.reachedTheGroup()
}

// StrangerOnMyGoal is the word of a touch where a stranger standing on Self's goal.
func StrangerOnMyGoal() cond.Word[Touch] {
	return cond.NewWord("stranger on my goal", Touch.strangerOnMyGoal)
}

func (t Touch) strangerOnMyGoal() bool { return !t.Ally && !t.OtherMoving && t.OnMyGoal }

// WaitsFirst is the word of a touch where Self is the one of two coming at each other that waits,
// the lower id,
// till its Hold runs out.
func WaitsFirst() cond.Word[Touch] { return cond.NewWord("waits first", Touch.waitsFirst) }

func (t Touch) waitsFirst() bool { return t.HeadOn && t.Self < t.Other && !t.WaitedOut }

// NoWayRound is the word of a touch where Self's Detour found no way round the one in its way.
func NoWayRound() cond.Word[Touch] { return cond.NewWord("no way round", Touch.noWayRound) }

func (t Touch) noWayRound() bool { return t.Moving && t.Cornered }

// WaitedLong is the word of a touch where Self's Hold ran out with the way still closed.
func WaitedLong() cond.Word[Touch] { return cond.NewWord("waited long", Touch.waitedLong) }

func (t Touch) waitedLong() bool { return t.WaitedOut }

// WalksMyWay is the word of a touch where the one in Self's way goes Self's way: under an order,
// not giving way, ahead of it on a route leading its way, and moving along or trailing someone.
func WalksMyWay() cond.Word[Touch] { return cond.NewWord("walks my way", Touch.walksMyWay) }

// one standing in a queue counts by its trailing; ahead within a quarter turn and going within
// one, two are never ahead of each other
func (t Touch) walksMyWay() bool {
	return t.Moving && !t.GivingWay && t.OtherMoving && !t.OtherGivingWay && t.Ahead &&
		t.SameWay && (t.Along || t.OtherTrailing)
}

// Groupmate is the word of a touch where one MoveTo sent the two.
func Groupmate() cond.Word[Touch] {
	return cond.NewWord("groupmate", func(t Touch) bool { return t.Groupmate })
}

var (
	_ host.Met     = Touch{}
	_ fact.Subject = Touch{}
)
