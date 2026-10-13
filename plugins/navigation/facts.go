package navigation

import (
	"time"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/rule/cond"
	"github.com/kjkrol/uid"
)

// Blocked is the fact told a unit with a plan while someone is in its way and a moment after:
// whom, the cell they hold, how the two stand to each other and what came of its commands.
type Blocked struct {
	By        uid.UID64
	Cell      cell.ID
	Stranger  bool
	Groupmate bool
	Moving    bool
	OnMyGoal  bool
	WaitedOut bool          // a Hold ran out, the way still closed
	Cornered  bool          // a Detour found no way round
	Lasts     time.Duration // how long it stays with nobody touched
}

// Subject is whom the unit is blocked by.
func (b Blocked) Subject() (uid.UID64, bool) { return b.By, true }

// Stuck is the word of a unit blocked whose Hold ran out with the way still closed.
func Stuck() cond.Word[Blocked] {
	return cond.NewWord("stuck", func(b Blocked) bool { return b.WaitedOut })
}

// Cornered is the word of a unit blocked whose Detour found no way round.
func Cornered() cond.Word[Blocked] {
	return cond.NewWord("cornered", func(b Blocked) bool { return b.Cornered })
}

// IdleAllyOnMyGoal is the word of a unit blocked by an ally with nothing to do standing on its
// goal.
func IdleAllyOnMyGoal() cond.Word[Blocked] {
	return cond.NewWord("idle ally on my goal",
		func(b Blocked) bool { return !b.Stranger && !b.Moving && b.OnMyGoal })
}

// IdleAlly is the word of a unit blocked by an ally with nothing to do.
func IdleAlly() cond.Word[Blocked] {
	return cond.NewWord("idle ally", func(b Blocked) bool { return !b.Stranger && !b.Moving })
}

// StrangerOnGoal is the word of a unit blocked by a stranger standing on its goal.
func StrangerOnGoal() cond.Word[Blocked] {
	return cond.NewWord("stranger on goal",
		func(b Blocked) bool { return b.Stranger && !b.Moving && b.OnMyGoal })
}

func blockedOf(t Touch, cell cell.ID) Blocked {
	return Blocked{By: t.Other, Cell: cell, Stranger: !t.Ally, Groupmate: t.Groupmate, Moving: t.OtherMoving,
		OnMyGoal: t.OnMyGoal, WaitedOut: t.WaitedOut, Cornered: t.Cornered, Lasts: blockedLasts}
}

// contacts come and go
const blockedLasts = 400 * time.Millisecond

// Arrived is the fact told a unit with a plan once its order is over, until the next: it stands
// on Cell, its goal or as near as it could come.
type Arrived struct{ Cell cell.ID }

// LastOrder is the group of the last MoveTo a unit came to the end of, zero for none; every unit
// carries it, and an order to give way leaves it as it was.
type LastOrder struct{ Group uint32 }
