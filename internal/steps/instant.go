package steps

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Pass is what a rule is told of the pass it fires in.
type Pass struct {
	Commands *control.Carrier
	Effects  *effect.Effects
	Dt       time.Duration
	Time     time.Duration // game time at the step's end
	Seed     uint64
	World    uid.UID64 // the world's own entity
	Around   func(moment any, rings int, each func(uid.UID64))
	Roles    func(id uid.UID64) uint64 // a bit a role
}

func (c *ctx) rolesOf(id uid.UID64) uint64 {
	roles := c.pass.Roles
	if c.sys != nil {
		roles = c.sys.roles
	}
	if roles == nil {
		return 0
	}
	return roles(id)
}

func (c *ctx) effects() *effect.Effects {
	if c.sys != nil {
		return c.sys.effects
	}
	return c.pass.Effects
}

type instantExec interface{ instant() }

// refuses says why not on the moment payload points to, empty where it runs
type momentExec interface{ refuses(payload any) string }

type placed interface{ Placed() }

// Instant is a rule laid out for running at a moment, every step of it done within the pass.
type Instant struct {
	tree *tree
	mind Mind
	c    ctx
}

// NewInstant lays root out as the rule named name, run on the moment payload points to. It
// panics on a step that lasts over ticks or one the moment cannot run.
func NewInstant(name string, root Step, payload any) *Instant {
	r := &Instant{tree: layOut(name, root)}
	for i, n := range r.tree.nodes {
		if _, ok := n.exec.(instantExec); !ok {
			panic(fmt.Sprintf("rule: %q: %s lasts over ticks — it belongs to a plan", name, r.tree.signs[i]))
		}
		if m, ok := n.exec.(momentExec); ok {
			if why := m.refuses(payload); why != "" {
				panic(fmt.Sprintf("rule: %q: %s %s, not %T", name, r.tree.signs[i], why, payload))
			}
		}
	}
	_, names := payload.(subject)
	r.c = ctx{instant: true, tree: r.tree, mind: &r.mind, payload: payload, names: names}
	return r
}

type subject interface{ Subject() (uid.UID64, bool) }

// Fire runs the rule once for id (entity false: for nobody), an Aimed command of it at subject
// when about.
func (r *Instant) Fire(p Pass, cb *goke.CmdBuf, id uid.UID64, entity bool, subject uid.UID64, about bool) {
	c := &r.c
	c.pass, c.cb, c.prev, c.next = p, cb, StepSet{}, StepSet{}
	c.id, c.me, c.entity, c.subject, c.about = id, id, entity, subject, about
	c.run(0)
}

// NewForOther runs step on each of the others the moment met, or on its subject, in place of
// the entity.
func NewForOther(step Step) Step {
	return composite{kids: []Step{step}, sign: "forother", make: func() exec { return forOther{} }}
}

// NewSame runs step on the entity itself, inside a step acting on others.
func NewSame(step Step) Step {
	return composite{kids: []Step{step}, sign: "self", make: func() exec { return same{} }}
}

type same struct{ basic }

func (same) instant() {}

func (same) tick(c *ctx, _ int, kids []int) Status {
	was := c.id
	c.id = c.me
	st := c.run(kids[0])
	c.id = was
	return st
}

// NewAround runs step on each place within rings of where the entity stands, in place of it.
func NewAround(rings int, step Step) Step {
	return composite{kids: []Step{step}, sign: fmt.Sprintf("around(%d)", rings), make: func() exec { return around{rings: rings} }}
}

type met interface{ Whom(each func(uid.UID64)) }

type around struct {
	basic
	rings int
}

func (around) instant() {}

func (around) refuses(payload any) string {
	if _, ok := payload.(placed); ok {
		return ""
	}
	return "needs a moment that is Placed"
}

func (a around) tick(c *ctx, _ int, kids []int) Status {
	if !c.entity || c.pass.Around == nil {
		return Failure
	}
	self, st := c.id, Failure
	c.pass.Around(c.payload, a.rings, func(place uid.UID64) {
		c.id = place
		if c.run(kids[0]) == Success {
			st = Success
		}
	})
	c.id = self
	return st
}

type forOther struct{ basic }

func (forOther) instant() {}

// in a plan the other is the subject of the fact the branch stands on
func (forOther) tick(c *ctx, _ int, kids []int) Status {
	if !c.entity {
		return Failure
	}
	self, st := c.id, Failure
	if !c.instant {
		if c.about {
			c.id = c.subject
			st = c.run(kids[0])
			c.id = self
		}
		return st
	}
	switch m := c.payload.(type) {
	case met:
		m.Whom(func(other uid.UID64) {
			c.id = other
			if c.run(kids[0]) == Success {
				st = Success
			}
		})
	case subject:
		if other, ok := m.Subject(); ok {
			c.id = other
			st = c.run(kids[0])
		}
	}
	c.id = self
	return st
}
