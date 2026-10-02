package navigation

import (
	"time"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

// Blocked is what navigation tells a unit with a tree (package rule) that touches someone
// in its way, for as long as it stays so and a moment after: whom, the cell they hold, and how the
// two stand to each other — strangers or allies (players/owner.Allies), of one group sent
// together, on the move or idle, on the unit's goal — and what came of the unit's commands: a
// Hold it waited out, a Detour with no way round. A rule learns the same from a Touch.
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

// WaitedLong: the unit held till its Hold ran out, and the way is still closed.
func (b Blocked) WaitedLong() bool { return b.WaitedOut }

// NoWayRound: the unit's Detour found no way round the one in its way.
func (b Blocked) NoWayRound() bool { return b.Cornered }

// IdleAllyOnMyGoal: an ally with nothing to do standing on the unit's goal.
func (b Blocked) IdleAllyOnMyGoal() bool { return !b.Stranger && !b.Moving && b.OnMyGoal }

// IdleAlly: an ally with nothing to do standing in the way.
func (b Blocked) IdleAlly() bool { return !b.Stranger && !b.Moving }

// StrangerOnMyGoal: a stranger standing on the unit's goal.
func (b Blocked) StrangerOnMyGoal() bool { return b.Stranger && !b.Moving && b.OnMyGoal }

// blockedOf is what t tells a unit with a tree, the other standing in cell.
func blockedOf(t Touch, cell cell.ID) Blocked {
	return Blocked{By: t.Other, Cell: cell, Stranger: !t.Ally, Groupmate: t.Groupmate, Moving: t.OtherMoving,
		OnMyGoal: t.OnMyGoal, WaitedOut: t.WaitedOut, Cornered: t.Cornered, Lasts: blockedLasts}
}

// blockedLasts is how long Blocked stays on a unit with nobody touched: contacts come and go.
const blockedLasts = 400 * time.Millisecond

// Arrived is what navigation tells a unit with a tree once its order is over, until the next: it
// stands on Cell, its goal or as near as it could come.
type Arrived struct{ Cell cell.ID }

// LastOrder is what every unit carries for good: the group of the last MoveTo it came to the end
// of — reached its goal, stopped short or gave up — zero for none. An order to give way leaves it
// as it was, so one stepped aside still belongs with its group (Touch.Groupmate).
type LastOrder struct{ Group uint32 }
