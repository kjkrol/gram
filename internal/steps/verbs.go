package steps

import (
	"fmt"
	"time"

	"github.com/kjkrol/gram/rule/effect"
)

func NewApply(e effect.Effect) Step {
	return leaf{sign: fmt.Sprintf("apply(%d)", e.Mark()), make: func() exec { return apply{e: e} }}
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
	c.effects().Cast(c.cb, c.id, a.e)
	return Success
}

func NewKeep(e effect.Effect) Step {
	return leaf{sign: fmt.Sprintf("keep(%d)", e.Mark()), make: func() exec { return hold{e: e} }}
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

const (
	keepCast = 1 + iota
	keepOn
)

func (h hold) tick(c *ctx, at int, _ []int) Status {
	if !c.entity {
		return Failure
	}
	if c.instant { // renewed every step, lapsing once the rule stops
		c.effects().CastFor(c.cb, c.id, h.e, 2*c.pass.Dt)
		return Success
	}
	on := c.sys.effects.Has(c.id, h.e)
	switch {
	case c.mind.Slot[at] == keepCast && on:
		c.mind.Slot[at] = keepOn
	case c.mind.Slot[at] == keepOn && !on:
		c.mind.Slot[at] = 0 // taken off by someone else
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

func NewDispel(e effect.Effect) Step {
	return leaf{sign: fmt.Sprintf("dispel(%d)", e.Mark()), make: func() exec { return dispel{e: e} }}
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
	c.effects().Dispel(c.id, d.e)
	return Success
}

func NewChance(p float64, step Step) Step {
	return composite{kids: []Step{step}, sign: fmt.Sprintf("chance(%g)", p), make: func() exec { return chance{p: p} }}
}

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
		t, seed = c.pass.Time, c.pass.Seed
	} else if c.sys != nil {
		seed = c.sys.seed
	}
	if draw(seed, t, uint64(c.id), hash(c.tree.name)+uint64(at)) >= ch.p {
		return Failure
	}
	return c.run(kids[0])
}

// in [0, 1), the same for the same seed, time, entity and place: a load and a replay draw alike
func draw(seed uint64, t time.Duration, id, salt uint64) float64 {
	x := seed ^ uint64(t)*0x9E3779B97F4A7C15 ^ id*0xBF58476D1CE4E5B9 ^ salt*0x94D049BB133111EB
	x ^= x >> 30
	x *= 0xBF58476D1CE4E5B9
	x ^= x >> 27
	x *= 0x94D049BB133111EB
	x ^= x >> 31
	return float64(x>>11) / (1 << 53)
}
