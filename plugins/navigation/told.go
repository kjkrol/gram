package navigation

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

// courtesyQueues are where the commands a unit's tree gives it among others land.
type courtesyQueues struct {
	detours control.Queue[Detour]
	holds   control.Queue[Hold]
	asides  control.Queue[StepAside]
	swaps   control.Queue[SwapGoals]
	settles control.Queue[Settle]
}

func (q *courtesyQueues) all() []control.CommandQueue {
	return []control.CommandQueue{&q.detours, &q.holds, &q.asides, &q.swaps, &q.settles}
}

// told is what a unit's tree commanded it this tick.
type told struct {
	detour, hold, aside, swap, settle bool
	stepAside                         StepAside
	swapWith, settleBeside            uid.UID64
}

// drain sorts the commands units gave themselves into by, by unit; a player's are none of these
// and are dropped.
func (q *courtesyQueues) drain(by map[uid.UID64]told) {
	q.detours.Drain(func(i control.Issued[Detour]) {
		if i.ByEntity {
			t := by[i.Entity]
			t.detour = true
			by[i.Entity] = t
		}
	})
	q.holds.Drain(func(i control.Issued[Hold]) {
		if i.ByEntity {
			t := by[i.Entity]
			t.hold = true
			by[i.Entity] = t
		}
	})
	q.asides.Drain(func(i control.Issued[StepAside]) {
		if i.ByEntity {
			t := by[i.Entity]
			t.aside, t.stepAside = true, i.Command
			by[i.Entity] = t
		}
	})
	q.swaps.Drain(func(i control.Issued[SwapGoals]) {
		if i.ByEntity {
			t := by[i.Entity]
			t.swap, t.swapWith = true, i.Command.With
			by[i.Entity] = t
		}
	})
	q.settles.Drain(func(i control.Issued[Settle]) {
		if i.ByEntity {
			t := by[i.Entity]
			t.settle, t.settleBeside = true, i.Command.Beside
			by[i.Entity] = t
		}
	})
}
