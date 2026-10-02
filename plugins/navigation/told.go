package navigation

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

// givenQueues are where the commands a unit gives itself among others land.
type givenQueues struct {
	asides  control.Queue[StepAside]
	detours control.Queue[Detour]
	passes  control.Queue[Pass]
	holds   control.Queue[Hold]
	settles control.Queue[Settle]
	stops   control.Queue[Stop]
}

func (q *givenQueues) all() []control.CommandQueue {
	return []control.CommandQueue{&q.asides, &q.detours, &q.passes, &q.holds, &q.settles, &q.stops}
}

// told is what a unit commanded itself this tick.
type told struct {
	aside, detour, pass, hold, settle, stop bool
	asideOf, detourOf, passOf, settleBeside uid.UID64
}

// drain sorts the commands units gave themselves into by, by unit; a player's are none of these
// and are dropped.
func (q *givenQueues) drain(by map[uid.UID64]told) {
	put := func(id uid.UID64, fn func(t *told)) {
		t := by[id]
		fn(&t)
		by[id] = t
	}
	q.asides.Drain(func(i control.Issued[StepAside]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.aside, t.asideOf = true, i.Command.Of })
		}
	})
	q.detours.Drain(func(i control.Issued[Detour]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.detour, t.detourOf = true, i.Command.Of })
		}
	})
	q.passes.Drain(func(i control.Issued[Pass]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.pass, t.passOf = true, i.Command.Of })
		}
	})
	q.holds.Drain(func(i control.Issued[Hold]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.hold = true })
		}
	})
	q.settles.Drain(func(i control.Issued[Settle]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.settle, t.settleBeside = true, i.Command.Beside })
		}
	})
	q.stops.Drain(func(i control.Issued[Stop]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.stop = true })
		}
	})
}
