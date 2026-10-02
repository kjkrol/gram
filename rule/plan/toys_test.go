package plan_test

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

// The toy commands, each done the ticks toys says after it is given.
type (
	walk struct{ Far int }
	flee struct{ Fast bool }
	jump struct{ High bool }
	rest struct{ Long bool }
)

// done is the fact toys tells an entity, for a tick, when what it did is done.
type done struct{ What uint8 }

// toys carries out the toy commands the entities give themselves: it notes what each does — a new
// command in place of the one before — and tells it done after the ticks it takes.
type toys struct {
	carrier control.Carrier
	walks   control.Queue[walk]
	flees   control.Queue[flee]
	jumps   control.Queue[jump]
	rests   control.Queue[rest]

	doing    map[uid.UID64]string
	last     map[uid.UID64]string // the last command each gave, done or not
	left     map[uid.UID64]int
	given    map[uid.UID64]int // how many commands each gave
	doneID   goke.CompID
	finished []uid.UID64 // told done last tick: the fact comes off
}

// ticks is how long each toy command takes.
var ticks = map[string]int{"walk": 3, "flee": 2, "jump": 2, "rest": 1}

func newToys() *toys {
	x := &toys{doing: map[uid.UID64]string{}, last: map[uid.UID64]string{}, left: map[uid.UID64]int{}, given: map[uid.UID64]int{}}
	if err := x.carrier.Carry(&x.walks, &x.flees, &x.jumps, &x.rests); err != nil {
		panic(err)
	}
	return x
}

func (x *toys) init(si *goke.SysInit) { x.doneID = si.RegComp[done]() }

// system counts down what the entities do, telling those done, then takes the new commands.
func (x *toys) system() goke.System {
	return goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, _ time.Duration) {
		for _, id := range x.finished {
			cb.RemoveCompOne(id, x.doneID)
		}
		x.finished = x.finished[:0]
		for id, n := range x.left {
			if n--; n > 0 {
				x.left[id] = n
				continue
			}
			delete(x.left, id)
			delete(x.doing, id)
			cb.AddOne(id, x.doneID, done{})
			x.finished = append(x.finished, id)
		}
		take := func(id uid.UID64, what string) {
			x.doing[id], x.last[id], x.left[id] = what, what, ticks[what]
			x.given[id]++
		}
		x.walks.Drain(func(i control.Issued[walk]) { take(i.Entity, "walk") })
		x.flees.Drain(func(i control.Issued[flee]) { take(i.Entity, "flee") })
		x.jumps.Drain(func(i control.Issued[jump]) { take(i.Entity, "jump") })
		x.rests.Drain(func(i control.Issued[rest]) { take(i.Entity, "rest") })
	}}
}
