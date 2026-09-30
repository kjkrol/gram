package act

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world/act/effect"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/uid"
)

// About is a moment of one entity: whose it is. A host's payload — a board.Standing, a
// vision.Sighting, a collision.Struck — is one; a clock.Moment is of no entity, and nodes acting on
// one fail on it.
type About interface{ Who() uid.UID64 }

// Met is a moment of one entity with others — whom it saw, whom it struck: the host matches it in
// pairs, and ToOther turns a node on the others.
type Met interface {
	About
	Whom(each func(uid.UID64))
}

// Reaction builds a trigger: a moment a host system catches — a unit standing on the board, one
// seeing another, two striking — and what is done then. Trigger makes it; Self, Other and Having
// narrow whom it fires for — with none it fires for every entity the host shows it, every pair
// for a moment that is Met; its methods make the nodes of its body, instant ones alone, each run
// within the host's pass; Do closes it round them, for the host's Hook: board.Plugin.Hook,
// vision's, collision's, world's. A trigger keeps no memory of its own: an effect's presence is
// its memory. A function handing back part of a body may use a Reaction of its own; its zero
// value will do.
type Reaction[P any] struct {
	name        string
	self, other host.Side
	paired      bool  // a Self or an Other was given
	others      bool  // an Other was given
	mine        *side // Self, for a moment that is not Met
	need        *having
}

// Trigger is a trigger of the moment P under name, to narrow and to give its body.
func Trigger[P any](name string) *Reaction[P] {
	return &Reaction[P]{name: name, self: host.AnySide, other: host.AnySide}
}

// side is a Self filter; on a moment that is not Met it reads the entity's tags as the trigger's
// state.
type side struct {
	tags    func() host.State
	carries func(state any) bool
}

// having is a Having filter.
type having struct {
	typ   reflect.Type
	state func() host.State
}

// Self fires the trigger only for an entity carrying t.
func (r *Reaction[P]) Self[F any](t tag.Tag[F]) *Reaction[P] {
	r.self, r.paired = host.SideOf(t), true
	r.mine = &side{tags: func() host.State { return host.StateOf[tag.Tags[F]]() },
		carries: func(state any) bool { return state.(*tag.Tags[F]).Has(t) }}
	return r
}

// Other fires the trigger only on a pair whose other entity carries t.
func (r *Reaction[P]) Other[F any](t tag.Tag[F]) *Reaction[P] {
	r.other, r.paired, r.others = host.SideOf(t), true, true
	return r
}

// Having fires the trigger only for an entity carrying the component T, and hands RunOn a
// pointer to it.
func (r *Reaction[P]) Having[T any]() *Reaction[P] {
	r.need = &having{typ: reflect.TypeFor[T](), state: func() host.State { return host.StateOf[T]() }}
	return r
}

// First runs its nodes in order and does as the first that does not fail; it fails when all fail.
func (r *Reaction[P]) First(nodes ...Instant) Instant { return quick{newFirst("", nodesOf(nodes)...)} }

// Then runs its nodes one after another while each does well; it fails with the first that fails.
func (r *Reaction[P]) Then(nodes ...Instant) Instant { return quick{newThen("", nodesOf(nodes)...)} }

// nodesOf is nodes as plain ones.
func nodesOf(nodes []Instant) []Node {
	out := make([]Node, len(nodes))
	for i, n := range nodes {
		out[i] = n
	}
	return out
}

// If runs node when holds says the moment holds, and fails otherwise.
func (r *Reaction[P]) If(holds func(P) bool, node Instant) Instant {
	return quick{newIf(holds, node)}
}

// Invert does well where node fails, and fails where it does well.
func (r *Reaction[P]) Invert(node Instant) Instant { return quick{newInvert(node)} }

// Apply casts the effect on the entity, lasting as its Spec says.
func (r *Reaction[P]) Apply(e effect.Effect) Instant { return quick{newApply(e)} }

// While holds the effect on the entity as long as the trigger keeps firing it: cast for two ticks
// at a time, it ends by itself when the trigger stops.
func (r *Reaction[P]) While(e effect.Effect) Instant { return quick{newWhile(e)} }

// Unless runs node while the entity is not under the effect, and fails while it is: "at most once
// a while" is Unless an effect lasting that while, applied in node.
func (r *Reaction[P]) Unless(e effect.Effect, node Instant) Instant {
	return quick{newUnless(e, node)}
}

// IfUnder runs node while the entity is under the effect, and fails while it is not.
func (r *Reaction[P]) IfUnder(e effect.Effect, node Instant) Instant {
	return quick{newIfUnder(e, node)}
}

// Issue gives the command cmd for the entity each time it fires — the same command a player gives
// — and does well at once.
func (r *Reaction[P]) Issue[C any](cmd C) Instant { return quick{newIssue(cmd)} }

// ToOther runs node on each of the others the moment met — whom the entity saw, whom it struck —
// in place of the entity: an effect applied there, a command issued for it.
func (r *Reaction[P]) ToOther(node Instant) Instant {
	return quick{composite{kids: []Node{node}, sign: "toother", make: func() step { return toOther{} }}}
}

// Run runs fn on the moment: a trigger's own code, where no node says it.
func (r *Reaction[P]) Run(fn func(t plugin.Tick, about P)) Instant {
	return quick{leaf{sign: "run", make: func() step { return runner[P]{fn: fn} }}}
}

// RunOn runs fn on the moment and the entity's T, which it may change: a trigger's own code over
// a component — the trigger fires only for entities carrying it.
func (r *Reaction[P]) RunOn[T any](fn func(t plugin.Tick, state *T, about P)) Instant {
	return quick{leaf{sign: "runon", make: func() step { return stateRunner[T, P]{fn: fn} }}}
}

// Runs is the trigger doing nothing but fn: Do(Run(fn)).
func (r *Reaction[P]) Runs(fn func(t plugin.Tick, about P)) plugin.Trigger { return r.Do(r.Run(fn)) }

// RunsOn is the trigger doing nothing but fn over the entity's T: Do(RunOn(fn)).
func (r *Reaction[P]) RunsOn[T any](fn func(t plugin.Tick, state *T, about P)) plugin.Trigger {
	return r.Do(r.RunOn(fn))
}

type runner[P any] struct {
	basic
	fn func(plugin.Tick, P)
}

func (runner[P]) instant() {}

func (r runner[P]) tick(c *ctx, _ int, _ []int) Status {
	if p, ok := c.payload.(*P); ok {
		r.fn(c.tick, *p)
		return Success
	}
	return Failure
}

type stateRunner[T, P any] struct {
	basic
	fn func(plugin.Tick, *T, P)
}

func (stateRunner[T, P]) instant() {}

func (r stateRunner[T, P]) needs() having {
	return having{typ: reflect.TypeFor[T](), state: func() host.State { return host.StateOf[T]() }}
}

func (r stateRunner[T, P]) tick(c *ctx, _ int, _ []int) Status {
	st, ok := c.state.(*T)
	p, okp := c.payload.(*P)
	if !ok || !okp {
		return Failure
	}
	r.fn(c.tick, st, *p)
	return Success
}

type toOther struct{ basic }

func (toOther) instant() {}

func (toOther) tick(c *ctx, _ int, kids []int) Status {
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

// instantStep is a step a trigger may run: done within its pass, keeping nothing.
type instantStep interface{ instant() }

// fired is a trigger's body, run on each moment its host hands it. Its mind is scratch: every node
// is entered afresh each firing, readying what it keeps.
type fired[P any] struct {
	tree    *tree
	current P
	about   bool // P is About an entity
	mind    Mind
	c       ctx
}

func (f *fired[P]) fire(t plugin.Tick, state any, about P) {
	f.current = about
	c := &f.c
	c.tick, c.cb, c.state, c.prev, c.next = t, t.CmdBuf, state, Nodes{}, Nodes{}
	c.id, c.entity = 0, f.about
	if f.about {
		c.id = any(&f.current).(About).Who() // a pointer: no copy to the heap
	}
	c.run(0)
}

// Do is the trigger with body — one node, or several one after another — for its host's Hook.
func (r *Reaction[P]) Do(body ...Instant) plugin.Trigger {
	if len(body) == 0 {
		panic(fmt.Sprintf("act: trigger %q does nothing", r.name))
	}
	var root Node = body[0]
	if len(body) > 1 {
		root = newThen(r.name, nodesOf(body)...)
	}
	f := &fired[P]{tree: layOut(r.name, root)}
	_, f.about = any(&f.current).(About)
	need := r.need
	for i, n := range f.tree.nodes {
		if _, ok := n.step.(instantStep); !ok {
			panic(fmt.Sprintf("act: trigger %q: %s lasts over ticks — it belongs to a kind's tree", r.name, f.tree.signs[i]))
		}
		if s, ok := n.step.(interface{ needs() having }); ok {
			h := s.needs()
			if need != nil && need.typ != h.typ {
				panic(fmt.Sprintf("act: trigger %q has %v and runs on %v", r.name, need.typ, h.typ))
			}
			need = &h
		}
	}
	f.c = ctx{instant: true, tree: f.tree, mind: &f.mind, payload: &f.current}
	_, met := any(*new(P)).(Met)
	switch {
	case need != nil:
		if r.paired {
			panic(fmt.Sprintf("act: trigger %q both narrows by tags and needs %v", r.name, need.typ))
		}
		return host.EachWith[P](need.state(), f.fire)
	case r.paired && !met:
		if r.mine == nil || r.others {
			panic(fmt.Sprintf("act: trigger %q: Other needs a moment with others (Met)", r.name))
		}
		carries := r.mine.carries
		return host.EachWith[P](r.mine.tags(), func(t plugin.Tick, state any, about P) {
			if carries(state) {
				f.fire(t, nil, about)
			}
		})
	case r.paired || met:
		return host.PairOf[P](r.self, r.other, func(t plugin.Tick, about P) { f.fire(t, nil, about) })
	}
	return host.Every[P](func(t plugin.Tick, about P) { f.fire(t, nil, about) })
}
