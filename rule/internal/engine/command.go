package engine

import (
	"fmt"

	"github.com/kjkrol/uid"
)

func NewOrder[C any](cmd C) Step {
	return leaf{sign: "order[" + typeName[C]() + "]", make: func() exec { return order[C]{cmd: cmd} }}
}

// Aimed is a command told whom it is about — the subject of the fact it stands under — as it is
// issued: whose way to step off, whose goal to swap with.
type Aimed interface{ Aim(who uid.UID64) }

type order[C any] struct {
	basic
	cmd C
}

func (order[C]) instant() {}

func (i order[C]) tick(c *ctx, _ int, _ []int) Status {
	if !c.entity {
		return Failure
	}
	v := i.cmd
	if a, ok := any(&v).(Aimed); ok && c.about {
		a.Aim(c.subject)
	} else if ok && c.names {
		return Failure // aimed at nobody
	}
	commands := c.pass.Commands
	if c.sys != nil {
		commands = c.sys.commands
	}
	if !commands.PutFrom(c.id, v) {
		panic(fmt.Sprintf("rule: no plugin handles %T — Use the one that defines it", v))
	}
	return Success
}

func NewUntil[F any](holds ...func(F) bool) Step {
	sign := "until[" + typeName[F]() + "]"
	var h func(F) bool
	if len(holds) > 0 {
		h, sign = holds[0], sign+funcName(holds[0])
	}
	return leaf{sign: sign, make: func() exec { return &until[F]{holds: h} }}
}

type until[F any] struct {
	holds func(F) bool
	fact  *fact[F]
}

func (u *until[F]) bind(s *system) { u.fact = factOf[F](s) }

func (u *until[F]) enter(c *ctx, at int) { c.mind.Slot[at] = 0 }
func (u *until[F]) halt(*ctx, int)       {}

func (u *until[F]) tick(c *ctx, at int, _ []int) Status {
	f := u.fact.of(c)
	if f == nil || (u.holds != nil && !u.holds(*f)) {
		c.mind.Slot[at] = 1 // not given: what comes from now on counts
		return Running
	}
	if c.mind.Slot[at] == 1 {
		return Success
	}
	return Running
}
