package navigation

import (
	"time"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/uid"
)

// The asks units put to each other (Ask in an act.Branch): make way for me — you close my way; free my
// goal — you cover it; and swap goals — I'll stand on yours.
type (
	MakeWay  struct{}
	FreeGoal struct{}
)

// Blocked is what navigation tells a unit with a tree that struck someone or had a step
// refused them, for as long as it stays so and a moment after: whom, the cell they hold, and how
// the two stand to each other — strangers or allies (players/owner.Allies), of one group sent
// together, on the move or idle, on the unit's goal, whether a swap of goals would shorten both
// ways, and, of two on the move, whether this one is the first to wait — and what came of the
// unit's commands: a Hold it waited out, a Detour with no way round.
type Blocked struct {
	By        uid.UID64
	Cell      board.CellID
	Stranger  bool
	Groupmate bool
	Moving    bool
	OnMyGoal  bool
	Shortens  bool
	First     bool
	Yielding  bool          // the unit itself is giving way: stepping aside, or going back
	WaitedOut bool          // a Hold ran out, the way still closed
	Cornered  bool          // a Detour found no way round
	Lasts     time.Duration // how long it stays with nobody struck
}

// Subject is whom the unit is blocked by.
func (b Blocked) Subject() (uid.UID64, bool) { return b.By, true }

// WhileGivingWay: the unit is giving way itself — stepping aside, or going back — and neither asks
// nor steps aside again: it waits, or goes round.
func (b Blocked) WhileGivingWay() bool { return b.Yielding }

// WaitedLong: the unit held till its Hold ran out, and the way is still closed.
func (b Blocked) WaitedLong() bool { return b.WaitedOut }

// NoWayRound: the unit's Detour found no way round the one in its way.
func (b Blocked) NoWayRound() bool { return b.Cornered }

// SwapShortens: one of the unit's group, whom a swap of goals would bring both nearer theirs —
// asked by the one of the two that does not wait first, so the two never ask each other at once.
func (b Blocked) SwapShortens() bool { return b.Groupmate && b.Shortens && !b.First }

// IdleAllyOnMyGoal: an ally with nothing to do standing on the unit's goal.
func (b Blocked) IdleAllyOnMyGoal() bool { return !b.Stranger && !b.Moving && b.OnMyGoal }

// IdleAlly: an ally with nothing to do standing in the way.
func (b Blocked) IdleAlly() bool { return !b.Stranger && !b.Moving }

// StrangerOnMyGoal: a stranger standing on the unit's goal.
func (b Blocked) StrangerOnMyGoal() bool { return b.Stranger && !b.Moving && b.OnMyGoal }

// GivesWayFirst: of two on the move, the one to wait while the other goes round — the lower id,
// the other way round when the two met before.
func (b Blocked) GivesWayFirst() bool { return b.Moving && b.First }

// Arrived is what navigation tells a unit with a tree, for a tick, when its order is over: it
// stands on Cell, its goal or as near as it could come.
type Arrived struct{ Cell board.CellID }

// Room is what navigation tells a unit with a tree, standing, that is asked to make way or
// to free a goal: whether it has somewhere free to step to — no farther than beside, no steeper
// than yieldClimb — or else the ally standing where it could, to ask on; a stranger's ask finds
// no room.
type Room struct {
	Free     bool
	Beside   bool // an ally stands where the unit could step: Ally
	Ally     uid.UID64
	Stranger bool
}

// Subject is the ally standing where the unit could step, when one does.
func (r Room) Subject() (uid.UID64, bool) { return r.Ally, r.Beside }

// IsFree: somewhere free to step to, for an ally.
func (r Room) IsFree() bool { return r.Free && !r.Stranger }

// TakenByAlly: nowhere free, but an ally stands where the unit could step, for an ally.
func (r Room) TakenByAlly() bool { return !r.Free && r.Beside && !r.Stranger }

// The commands a unit's tree gives it among others (Issue in an act.Branch), carried out for the unit
// that gives them; those that name whom they are about are told by the tree (act.Aimed).
type (
	// Detour notes the cell of whoever blocks the unit and plans a way round it; with none,
	// Blocked says it is cornered.
	Detour struct{}
	// Hold keeps the unit where it stands till the way ahead clears, stallAfter at most; then
	// Blocked says it waited out.
	Hold struct{}
	// StepAside has the unit step off the way of Of, beside: standing, and back home after a
	// while when Return; on the move, a while, then on to its own goals.
	StepAside struct {
		Return bool
		Of     uid.UID64
	}
	// SwapGoals swaps the unit's goal with With's, both of one group.
	SwapGoals struct{ With uid.UID64 }
	// Settle has the unit stand beside its goal, Beside on it.
	Settle struct{ Beside uid.UID64 }
)

// Aim tells the command whose way it is.
func (s *StepAside) Aim(who uid.UID64) { s.Of = who }

// Aim tells the command whose goal to swap with.
func (s *SwapGoals) Aim(who uid.UID64) { s.With = who }

// Aim tells the command who stands on the goal.
func (s *Settle) Aim(who uid.UID64) { s.Beside = who }

// Courteous is how a player's unit travels among others: it answers its allies' asks, swaps goals
// within its group, asks allies in its way to make way, and goes round strangers.
func Courteous() act.Node {
	c := act.Named("navigation.courteous")
	return c.Do(c.First(
		MakeWayWhenAsked(),
		LeaveTheGoalWhenAsked(),
		SwapWhenAsked(),
		WhenBlocked(),
	))
}

// MakeWayWhenAsked steps aside and comes back when an ally asks, asks an ally beside on when
// there is no room, and refuses otherwise.
func MakeWayWhenAsked() act.Node {
	c := act.On[act.Asked[MakeWay]]("make way when asked")
	return c.Do(c.First(
		c.If(Room.IsFree, c.Then(c.Agree[MakeWay](), c.Issue(StepAside{Return: true}))),
		c.If(Room.TakenByAlly, c.Relay[MakeWay]()),
		c.Refuse[MakeWay](),
	))
}

// LeaveTheGoalWhenAsked steps off an ally's goal for good, asking an ally beside on when there is
// no room, and refuses otherwise.
func LeaveTheGoalWhenAsked() act.Node {
	c := act.On[act.Asked[FreeGoal]]("leave the goal when asked")
	return c.Do(c.First(
		c.If(Room.IsFree, c.Then(c.Agree[FreeGoal](), c.Issue(StepAside{}))),
		c.If(Room.TakenByAlly, c.Relay[FreeGoal]()),
		c.Refuse[FreeGoal](),
	))
}

// SwapWhenAsked agrees to swap goals with one of its group.
func SwapWhenAsked() act.Node {
	c := act.On[act.Asked[SwapGoals]]("swap goals when asked")
	return c.Do(c.Agree[SwapGoals]())
}

// WhenBlocked answers someone in the way. A unit giving way itself waits, then goes round. One
// cornered steps aside until the other has passed; one that waited out goes round. A groupmate a
// swap helps is asked to swap goals, an idle ally on the goal to free it — else the unit stands
// beside it — an idle ally in the way to make way — else the unit goes round; a stranger on the
// goal has the unit stand beside it at once; of two on the move the first waits and the other
// goes round, and any stranger is gone round at once. Each command is given once, as its branch
// begins, and the branch stays till what comes of it — the fact changing — has another take over:
// the reaction to what came of a command stands earlier in the First than the command.
func WhenBlocked() act.Node {
	c := act.When[Blocked]("when blocked")
	hold := c.Issue(Hold{}).Until(Blocked.WaitedLong)
	goRound := c.Issue(Detour{}).Until(Blocked.NoWayRound)
	return c.Do(c.First(
		c.If(Blocked.WhileGivingWay, c.First(c.If(Blocked.WaitedLong, goRound), hold)),
		c.If(Blocked.NoWayRound, c.Issue(StepAside{Return: true}).Stay()),
		c.If(Blocked.WaitedLong, goRound),
		c.If(Blocked.SwapShortens, c.Ask[SwapGoals]("I'll stand on yours", time.Second, c.Issue(SwapGoals{}).Stay())),
		c.If(Blocked.IdleAllyOnMyGoal, c.Ask[FreeGoal]("you cover my goal", time.Second, hold, c.Issue(Settle{}).Stay())),
		c.If(Blocked.IdleAlly, c.Ask[MakeWay]("you close my way", time.Second, hold, goRound)),
		c.If(Blocked.StrangerOnMyGoal, c.Issue(Settle{}).Stay()),
		c.If(Blocked.GivesWayFirst, hold),
		goRound,
	))
}
