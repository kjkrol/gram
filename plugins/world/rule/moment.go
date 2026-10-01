package rule

import (
	"fmt"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/rule/effect"
	"github.com/kjkrol/uid"
)

// About is a moment of one entity: whose it is. A host's payload — a board.Standing, a
// vision.Sighting, a collision.Struck, a clock.Moment (the clock's own entity) — is one; on a moment
// of no entity steps acting on one fail.
type About interface{ Who() uid.UID64 }

// Met is a moment of one entity with others — whom it saw, whom it struck: the host matches it in
// pairs, and ForOther turns a step on the others.
type Met interface {
	About
	Whom(each func(uid.UID64))
}

// Placed is a moment of an entity standing somewhere, on places that are entities of their own —
// a board's cells — whose host tells, in the Tick, which places lie round it (plugin.Tick.Around):
// Here and Around turn a step on them. The moment is data alone.
type Placed interface {
	About
	Placed()
}

// On is a rule, named name: at every moment P a plugin's pass catches — a unit standing on the
// board, one seeing another, two striking — for whom filter lets through, it runs the steps body
// writes for the Moment, steps done within the pass alone. Hook it on the plugin that catches P:
// board.Plugin.Hook, vision's, collision's, world's, navigation's. A rule keeps no memory of its
// own: an effect's presence is its memory.
func On[P any](name string, filter Filter, body func(m *Moment[P]) Step) plugin.Rule {
	return build[P](name, filter, body(&Moment[P]{}))
}

// Filter is whom a rule fires for: All, Self, Between or Having.
type Filter struct {
	self, other host.Side
	paired      bool  // Self or Between
	others      bool  // Between
	mine        *side // Self, for a moment that is not Met
	need        *having
}

// All lets every entity the plugin shows the rule through — every pair, for a moment that is Met.
var All Filter

// Self lets through an entity carrying t.
func Self[F any](t tag.Tag[F]) Filter {
	return Filter{self: host.SideOf(t), other: host.AnySide, paired: true,
		mine: &side{tags: func() host.State { return host.StateOf[tag.Tags[F]]() },
			carries: func(state any) bool { return state.(*tag.Tags[F]).Has(t) }}}
}

// Between lets through a pair whose entity carries a and whose other carries b — tag.Any for
// either — for a moment that is Met.
func Between[FA, FB any](a tag.Tag[FA], b tag.Tag[FB]) Filter {
	return Filter{self: host.SideOf(a), other: host.SideOf(b), paired: true, others: true}
}

// Having lets through an entity carrying the component T.
func Having[T any]() Filter {
	return Filter{need: &having{state: func() host.State { return host.StateOf[T]() }}}
}

// side is a Self filter; on a moment that is not Met it reads the entity's tags as the rule's
// state.
type side struct {
	tags    func() host.State
	carries func(state any) bool
}

// having is a Having filter.
type having struct{ state func() host.State }

// Moment is the moment P a rule is written for: its methods make the steps the rule takes then,
// each done within the plugin's pass. On hands one to the rule's body; a function writing part of
// a rule takes it as its own.
type Moment[P any] struct{}

// OneOf runs its steps in order and does as the first that does not fail; it fails when all fail.
func (m *Moment[P]) OneOf(steps ...Step) Step { return newFirst("", steps...) }

// Steps runs its steps one after another while each does well; it fails with the first that fails.
func (m *Moment[P]) Steps(steps ...Step) Step { return newThen("", steps...) }

// If runs step when holds says the moment holds, and fails otherwise.
func (m *Moment[P]) If(holds func(P) bool, step Step) Step { return newIf(holds, step) }

// Not does well where step fails, and fails where it does well.
func (m *Moment[P]) Not(step Step) Step { return newInvert(step) }

// Apply casts the effect on the entity, lasting as its Spec says.
func (m *Moment[P]) Apply(e effect.Effect) Step { return newApply(e) }

// Keep holds the effect on the entity as long as the rule keeps firing it: cast for two ticks at
// a time, it ends by itself when the rule stops.
func (m *Moment[P]) Keep(e effect.Effect) Step { return newKeep(e) }

// Dispel takes the effect off the entity with the effects' next pass, and does well; a rule
// keeping it may cast it again.
func (m *Moment[P]) Dispel(e effect.Effect) Step { return newDispel(e) }

// Chance runs step with likelihood p, drawn afresh at every step of the game from the world's seed,
// the moment's game time and the entity, and fails otherwise: the same after a load and in a replay.
func (m *Moment[P]) Chance(p float64, step Step) Step { return newChance(p, step) }

// Unless runs step while the entity is not under the effect, and fails while it is: "at most once
// a while" is Unless an effect lasting that while, applied in step.
func (m *Moment[P]) Unless(e effect.Effect, step Step) Step { return newUnless(e, step) }

// Under runs step while the entity is under the effect, and fails while it is not.
func (m *Moment[P]) Under(e effect.Effect, step Step) Step { return newUnder(e, step) }

// During runs step while the world is under the effect — a state of the whole game, a lever
// pulled, an alarm (world.Apply) — and fails while it is not.
func (m *Moment[P]) During(e effect.Effect, step Step) Step { return newDuring(e, step) }

// Order gives the command cmd for the entity each time it fires — the same command a player gives
// — and does well at once; one that is Aimed is told the moment's Subject, when it names one.
func (m *Moment[P]) Order[C any](cmd C) Step { return newOrder(cmd) }

// ForOther runs step on each of the others the moment met — whom the entity saw, whom it struck —
// in place of the entity: an effect applied there, a command ordered for it.
func (m *Moment[P]) ForOther(step Step) Step {
	return composite{kids: []Step{step}, sign: "forother", make: func() exec { return forOther{} }}
}

// Here runs step on each place the entity stands on — the cells under it — in place of the entity:
// an effect applied to the ground. The moment must be Placed.
func (m *Moment[P]) Here(step Step) Step { return m.Around(0, step) }

// Around runs step on each place within rings of where the entity stands, those it stands on
// among them, in place of the entity. The moment must be Placed.
func (m *Moment[P]) Around(rings int, step Step) Step {
	if _, ok := any(*new(P)).(Placed); !ok {
		panic(fmt.Sprintf("rule: Here and Around need a moment that is Placed, not %T", *new(P)))
	}
	return composite{kids: []Step{step}, sign: fmt.Sprintf("around(%d)", rings), make: func() exec { return around{rings: rings} }}
}

type around struct {
	basic
	rings int
}

func (around) instant() {}

func (a around) tick(c *ctx, _ int, kids []int) Status {
	if !c.entity || c.tick.Around == nil {
		return Failure
	}
	self, st := c.id, Failure
	c.tick.Around(c.payload, a.rings, func(place uid.UID64) {
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
	m, ok := c.payload.(Met)
	if !ok || !c.entity {
		return Failure
	}
	self, st := c.id, Failure
	m.Whom(func(other uid.UID64) {
		c.id = other
		if c.run(kids[0]) == Success {
			st = Success
		}
	})
	c.id = self
	return st
}

// instantExec is a step a rule may run: done within its pass, keeping nothing.
type instantExec interface{ instant() }

// fired is a rule's body, run on each moment its plugin hands it. Its mind is scratch: every step
// is entered afresh each firing, readying what it keeps.
type fired[P any] struct {
	tree    *tree
	current P
	about   bool // P is About an entity
	subject bool // P names a Subject, whom an Aimed command is about
	mind    Mind
	c       ctx
}

func (f *fired[P]) fire(t plugin.Tick, _ any, about P) {
	f.current = about
	c := &f.c
	c.tick, c.cb, c.prev, c.next = t, t.CmdBuf, StepSet{}, StepSet{}
	c.id, c.entity = 0, f.about
	if f.about {
		c.id = any(&f.current).(About).Who() // a pointer: no copy to the heap
	}
	c.subject, c.about = 0, false
	if f.subject {
		c.subject, c.about = any(&f.current).(Subject).Subject()
	}
	c.run(0)
}

// build is the rule named name over root, for whom filter lets through, as its plugin's host
// takes it.
func build[P any](name string, filter Filter, root Step) plugin.Rule {
	f := &fired[P]{tree: layOut(name, root)}
	_, f.about = any(&f.current).(About)
	_, f.subject = any(&f.current).(Subject)
	for i, n := range f.tree.nodes {
		if _, ok := n.exec.(instantExec); !ok {
			panic(fmt.Sprintf("rule: %q: %s lasts over ticks — it belongs to a plan", name, f.tree.signs[i]))
		}
	}
	f.c = ctx{instant: true, tree: f.tree, mind: &f.mind, payload: &f.current}
	_, met := any(*new(P)).(Met)
	switch {
	case filter.need != nil:
		return host.EachWith[P](filter.need.state(), f.fire)
	case filter.paired && !met:
		if filter.mine == nil || filter.others {
			panic(fmt.Sprintf("rule: %q: Between needs a moment with others (Met)", name))
		}
		carries := filter.mine.carries
		return host.EachWith[P](filter.mine.tags(), func(t plugin.Tick, state any, about P) {
			if carries(state) {
				f.fire(t, nil, about)
			}
		})
	case filter.paired || met:
		return host.PairOf[P](filter.self, filter.other, func(t plugin.Tick, about P) { f.fire(t, nil, about) })
	}
	return host.Every[P](func(t plugin.Tick, about P) { f.fire(t, nil, about) })
}
