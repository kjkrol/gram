package steps

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Pass is what a rule is told of the pass it fires in: the carrier its commands go to, the world's
// effects, the tick's length, the game time the step ends at, the world's seed and own entity,
// and the places round a moment, its system's to say.
type Pass struct {
	Commands *control.Carrier
	Effects  *effect.Effects
	Dt       time.Duration
	Time     time.Duration
	Seed     uint64
	World    uid.UID64
	Around   func(moment any, rings int, each func(uid.UID64))
	Roles    func(id uid.UID64) uint64 // the roles an entity plays, a bit each
}

// rolesOf is the roles id plays: the plans' lookup, or the one the rule's pass was given.
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

// effects is the world's effects: the plans' own, or those the rule's pass was given.
func (c *ctx) effects() *effect.Effects {
	if c.sys != nil {
		return c.sys.effects
	}
	return c.pass.Effects
}

// instantExec is a step a rule may run: done within its pass, keeping nothing.
type instantExec interface{ instant() }

// momentExec is a step that runs on some moments alone: refuses says why not on the one payload
// points to, empty where it runs.
type momentExec interface{ refuses(payload any) string }

// placed is a moment standing on places of their own (plugin.Placed).
type placed interface{ Placed() }

// Instant is a rule laid out for running at a moment, every step of it done within the pass; its
// mind is scratch, every step entered afresh each firing.
type Instant struct {
	tree *tree
	mind Mind
	c    ctx
}

// NewInstant lays root out as the rule named name, run on the moment payload points to; it panics
// on a step that lasts over ticks and on one the moment cannot run.
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

// subject is a moment that names whom it is about, or nobody just now.
type subject interface{ Subject() (uid.UID64, bool) }

// Fire runs the rule once: for id when entity says it acts for one, an Aimed command of it at
// subject when about.
func (r *Instant) Fire(p Pass, cb *goke.CmdBuf, id uid.UID64, entity bool, subject uid.UID64, about bool) {
	c := &r.c
	c.pass, c.cb, c.prev, c.next = p, cb, StepSet{}, StepSet{}
	c.id, c.entity, c.subject, c.about = id, entity, subject, about
	c.run(0)
}

// NewForOther runs step on each of the others the moment met — or on its subject, for a moment of
// one entity naming one — in place of the entity.
func NewForOther(step Step) Step {
	return composite{kids: []Step{step}, sign: "forother", make: func() exec { return forOther{} }}
}

// NewAround runs step on each place within rings of where the entity stands, in place of it.
func NewAround(rings int, step Step) Step {
	return composite{kids: []Step{step}, sign: fmt.Sprintf("around(%d)", rings), make: func() exec { return around{rings: rings} }}
}

// met is a moment of one entity with others.
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

func (forOther) tick(c *ctx, _ int, kids []int) Status {
	if !c.entity {
		return Failure
	}
	self, st := c.id, Failure
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
