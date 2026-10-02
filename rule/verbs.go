package rule

import (
	"fmt"
	"time"

	"github.com/kjkrol/gram/rule/effect"
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
	c.mind.Slot[at] = keepCast
}

// A plan's Keep in its Slot: cast, or seen on the actor since — off then, someone took it off.
const (
	keepCast = 1 + iota
	keepOn
)

func (h hold) tick(c *ctx, at int, _ []int) Status {
	if !c.entity {
		return Failure
	}
	if c.instant {
		h.e.CastFor(c.cb, c.id, 2*c.tick.Dt)
		return Success
	}
	on := c.sys.effects.Has(c.id, h.e)
	switch {
	case c.mind.Slot[at] == keepCast && on:
		c.mind.Slot[at] = keepOn
	case c.mind.Slot[at] == keepOn && !on:
		c.mind.Slot[at] = 0 // taken off by someone else: the branch gives way
		return Failure
	}
	return Running
}

func (h hold) halt(c *ctx, at int) {
	if !c.instant && c.mind.Slot[at] != 0 {
		c.sys.effects.Dispel(c.id, h.e)
		c.mind.Slot[at] = 0
	}
}

func newDispel(e effect.Effect) Step {
	return leaf{sign: fmt.Sprintf("dispel(%d)", e.ID()), make: func() exec { return dispel{e: e} }}
}

type dispel struct {
	basic
	e effect.Effect
}

func (dispel) instant() {}

func (d dispel) tick(c *ctx, _ int, _ []int) Status {
	if !c.entity {
		return Failure
	}
	if c.instant {
		d.e.Dispel(c.id)
	} else {
		c.sys.effects.Dispel(c.id, d.e)
	}
	return Success
}

func newChance(p float64, step Step) Step {
	return composite{kids: []Step{step}, sign: fmt.Sprintf("chance(%g)", p), make: func() exec { return chance{p: p} }}
}

// chance runs its step with likelihood p, drawn afresh at every step of the game, and fails
// otherwise.
type chance struct {
	basic
	p float64
}

func (chance) instant() {}

func (ch chance) tick(c *ctx, at int, kids []int) Status {
	if !c.entity {
		return Failure
	}
	t, seed := c.now, uint64(0)
	if c.instant {
		t, seed = c.tick.Time, c.tick.Seed
	} else if c.sys != nil {
		seed = c.sys.seed
	}
	if draw(seed, t, uint64(c.id), hash(c.tree.name)+uint64(at)) >= ch.p {
		return Failure
	}
	return c.run(kids[0])
}

// draw is a number in [0, 1), the same for the same seed, time, entity and place in a rule or a
// plan, and nothing kept: a load and a replay draw alike.
func draw(seed uint64, t time.Duration, id, salt uint64) float64 {
	x := seed ^ uint64(t)*0x9E3779B97F4A7C15 ^ id*0xBF58476D1CE4E5B9 ^ salt*0x94D049BB133111EB
	x ^= x >> 30
	x *= 0xBF58476D1CE4E5B9
	x ^= x >> 27
	x *= 0x94D049BB133111EB
	x ^= x >> 31
	return float64(x>>11) / (1 << 53)
}

func newDuring(e effect.Effect, node Step) Step {
	return composite{kids: []Step{node}, sign: fmt.Sprintf("during(%d)", e.ID()), make: func() exec { return during{e: e} }}
}

// during runs its step while the world — its own entity, the clock's — is under the effect.
type during struct {
	basic
	e effect.Effect
}

func (during) instant() {}

func (d during) tick(c *ctx, _ int, kids []int) Status {
	on := false
	if c.instant {
		on = d.e.On(c.tick.World)
	} else if c.sys.world != nil {
		on = c.sys.effects.Has(c.sys.world(), d.e)
	}
	if !on {
		return Failure
	}
	return c.run(kids[0])
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
