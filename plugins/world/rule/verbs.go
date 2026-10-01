package rule

import (
	"fmt"

	"github.com/kjkrol/gram/plugins/world/rule/effect"
)

func newApply(e effect.Effect) Step {
	return leaf{sign: fmt.Sprintf("apply(%d)", e.ID()), make: func() exec { return apply{e: e} }}
}

type apply struct {
	basic
	e effect.Effect
}

func (apply) instant() {}

func (a apply) tick(c *ctx, _ int, _ []int) Status {
	if !c.entity {
		return Failure
	}
	if c.instant {
		a.e.Cast(c.cb, c.id)
	} else {
		c.sys.effects.Cast(c.cb, c.id, a.e)
	}
	return Success
}

func newKeep(e effect.Effect) Step {
	return leaf{sign: fmt.Sprintf("keep(%d)", e.ID()), make: func() exec { return hold{e: e} }}
}

type hold struct{ e effect.Effect }

func (hold) instant() {}

func (h hold) enter(c *ctx, at int) {
	if c.instant {
		return
	}
	c.sys.effects.CastFor(c.cb, c.id, h.e, effect.Forever)
	c.mind.Slot[at] = 1
}

func (h hold) tick(c *ctx, _ int, _ []int) Status {
	if !c.entity {
		return Failure
	}
	if c.instant {
		h.e.CastFor(c.cb, c.id, 2*c.tick.Dt)
		return Success
	}
	return Running
}

func (h hold) halt(c *ctx, at int) {
	if !c.instant && c.mind.Slot[at] == 1 {
		c.sys.effects.Dispel(c.id, h.e)
		c.mind.Slot[at] = 0
	}
}

func newUnless(e effect.Effect, node Step) Step {
	return composite{kids: []Step{node}, sign: fmt.Sprintf("unless(%d)", e.ID()), make: func() exec { return under{e: e, not: true} }}
}

func newUnder(e effect.Effect, node Step) Step {
	return composite{kids: []Step{node}, sign: fmt.Sprintf("under(%d)", e.ID()), make: func() exec { return under{e: e} }}
}

type under struct {
	basic
	e   effect.Effect
	not bool
}

func (under) instant() {}

func (u under) tick(c *ctx, _ int, kids []int) Status {
	if !c.entity {
		return Failure
	}
	on := false
	if c.instant {
		on = u.e.On(c.id)
	} else {
		on = c.sys.effects.Has(c.id, u.e)
	}
	if on == u.not {
		return Failure
	}
	return c.run(kids[0])
}
