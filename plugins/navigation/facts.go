package navigation

import (
	"time"

	"github.com/kjkrol/gram/plugins/board/cell"
)

func blockedOf(t Touch, cell cell.ID) Blocked {
	return Blocked{By: t.Other, Cell: cell, Stranger: !t.Ally, Groupmate: t.Groupmate, Moving: t.OtherMoving,
		OnMyGoal: t.OnMyGoal, WaitedOut: t.WaitedOut, Cornered: t.Cornered, Lasts: blockedLasts}
}

// contacts come and go
const blockedLasts = 400 * time.Millisecond
