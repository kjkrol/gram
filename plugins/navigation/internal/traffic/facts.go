package traffic

import (
	"time"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
)

func blockedOf(t order.Touch, cell cell.ID) order.Blocked {
	return order.Blocked{By: t.Other, Cell: cell, Stranger: !t.Ally, Groupmate: t.Groupmate, Moving: t.OtherMoving,
		OnMyGoal: t.OnMyGoal, WaitedOut: t.WaitedOut, Cornered: t.Cornered, Lasts: blockedLasts}
}

// contacts come and go
const blockedLasts = 400 * time.Millisecond
