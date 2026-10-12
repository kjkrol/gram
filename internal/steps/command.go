package steps

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/uid"
)

// NewOrder gives the entity the command cmd, or sends it as a message where cmd carries a From.
func NewOrder[C control.Command](cmd C) Step {
	return leaf{sign: "order[" + typeName[C]() + "]",
		make: func() exec { return &order[C]{cmd: cmd} }}
}

// Aimed is a command told the subject of the fact or moment it stands under as it is issued.
type Aimed interface{ Aim(who uid.UID64) }

type order[C control.Command] struct {
	basic
	cmd  C
	fact *fact[C] // a message's
}

func (*order[C]) instant() {}

func (i *order[C]) sends() bool {
	var v C
	_, ok := any(&v).(message)
	return ok
}

func (i *order[C]) refuses(any) string {
	if i.sends() {
		return "sends a message, which only a plan can"
	}
	return ""
}

func (i *order[C]) bind(s *system) {
	if i.sends() {
		i.fact = factOf[C](s)
	}
}

func (i *order[C]) loads() []goke.CompToken {
	if !i.sends() {
		return nil
	}
	return []goke.CompToken{goke.LoadComp[C]()}
}

func (i *order[C]) tick(c *ctx, _ int, _ []int) Status {
	if !c.entity {
		return Failure
	}
	v := i.cmd
	if m, ok := any(&v).(message); ok {
		return i.send(c, &v, m.from())
	}
	if a, ok := any(&v).(Aimed); ok && c.about {
		a.Aim(c.subject)
	} else if ok && c.names {
		return Failure
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

// to whom the branch acts on, never to the sender itself; a message passed on keeps count of
// its hops, one of another type begins its own count
func (i *order[C]) send(c *ctx, v *C, from *From) Status {
	if c.id == c.me {
		return Failure
	}
	var hops uint8
	if c.hopsOf == reflect.TypeFor[C]() {
		hops = c.hops + 1
	}
	if hops >= MaxChain {
		return Failure
	}
	*from = From{By: c.me, At: c.now, Hops: hops}
	if !i.fact.putOn(c, c.id, *v) {
		return Failure
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
