package traffic

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/uid"
)

// GivenQueues are the commands the units give themselves among others.
type GivenQueues struct {
	asides  command.Queue[order.StepAside]
	detours command.Queue[order.Detour]
	passes  command.Queue[order.Pass]
	holds   command.Queue[order.Hold]
	settles command.Queue[order.Settle]
	stops   command.Queue[order.Stop]
	trails  command.Queue[order.Trail]
}

// All are the queues, for the carrier.
func (q *GivenQueues) All() []command.CommandQueue {
	return []command.CommandQueue{&q.asides, &q.detours, &q.passes, &q.holds, &q.settles, &q.stops,
		&q.trails}
}

type told struct {
	aside, detour, pass, hold, settle, stop, trail   bool
	asideOf, detourOf, passOf, settleBeside, trailOf uid.UID64
}

// a player's are dropped
func (q *GivenQueues) drain(by map[uid.UID64]told) {
	put := func(id uid.UID64, fn func(t *told)) {
		t := by[id]
		fn(&t)
		by[id] = t
	}
	q.asides.Drain(func(i command.Issued[order.StepAside]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.aside, t.asideOf = true, i.Command.Of })
		}
	})
	q.detours.Drain(func(i command.Issued[order.Detour]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.detour, t.detourOf = true, i.Command.Of })
		}
	})
	q.passes.Drain(func(i command.Issued[order.Pass]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.pass, t.passOf = true, i.Command.Of })
		}
	})
	q.holds.Drain(func(i command.Issued[order.Hold]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.hold = true })
		}
	})
	q.settles.Drain(func(i command.Issued[order.Settle]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.settle, t.settleBeside = true, i.Command.Beside })
		}
	})
	q.stops.Drain(func(i command.Issued[order.Stop]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.stop = true })
		}
	})
	q.trails.Drain(func(i command.Issued[order.Trail]) {
		if i.ByEntity {
			put(i.Entity, func(t *told) { t.trail, t.trailOf = true, i.Command.Of })
		}
	})
}
