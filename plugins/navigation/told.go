package navigation

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/uid"
)

type givenQueues struct {
	asides  command.Queue[StepAside]
	detours command.Queue[Detour]
	passes  command.Queue[Pass]
	holds   command.Queue[Hold]
	settles command.Queue[Settle]
	stops   command.Queue[Stop]
}

func (q *givenQueues) all() []command.CommandQueue {
	return []command.CommandQueue{&q.asides, &q.detours, &q.passes, &q.holds, &q.settles, &q.stops}
}

type told struct {
	aside, detour, pass, hold, settle, stop bool
	asideOf, detourOf, passOf, settleBeside uid.UID64
}

// a player's are dropped
func (q *givenQueues) drain(by map[uid.UID64]told) {
	put := func(id uid.UID64, fn func(t *told)) {
		t := by[id]
		fn(&t)
		by[id] = t
	}
	q.asides.Drain(func(i command.Issued[StepAside]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.aside, t.asideOf = true, i.Command.Of })
		}
	})
	q.detours.Drain(func(i command.Issued[Detour]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.detour, t.detourOf = true, i.Command.Of })
		}
	})
	q.passes.Drain(func(i command.Issued[Pass]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.pass, t.passOf = true, i.Command.Of })
		}
	})
	q.holds.Drain(func(i command.Issued[Hold]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.hold = true })
		}
	})
	q.settles.Drain(func(i command.Issued[Settle]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.settle, t.settleBeside = true, i.Command.Beside })
		}
	})
	q.stops.Drain(func(i command.Issued[Stop]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.stop = true })
		}
	})
}
