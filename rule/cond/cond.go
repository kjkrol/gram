package cond

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Cond is a branch's When: clauses of who is in what state, joined with And and Or.
type Cond interface {
	// And holds where both hold.
	And(c Cond) Cond
	// Or holds where either holds.
	Or(c Cond) Cond
	cond()
}

// Subject is whom a clause is about: Self, Other, Around(n) or On(plugin). If makes the
// clause.
type Subject struct{ who who }

var (
	// Self is the entity the branch is about.
	Self = Subject{who{kind: ofSelf}}
	// Other is the other of the pair the moment met, or the subject of the moment or the fact;
	// a clause of it does not hold while there is none.
	Other = Subject{who{kind: ofOther}}
)

// Around is each place within rings of where the entity stands, in a rule of a moment that is
// plugin.Placed: the branch runs once a place.
func Around(rings int) Subject { return Subject{who{kind: ofAround, rings: rings}} }

// On is a plugin's own entity: anything with an Entity.
func On(plugin any) Subject {
	w := who{kind: ofEntity}
	if p, ok := plugin.(interface{ Entity() uid.UID64 }); ok {
		w.of = p.Entity
	} else {
		w.bad = fmt.Sprintf("%T is no plugin: On takes one with an Entity", plugin)
	}
	return Subject{w}
}

// If is the subject being in the state what: under an effect, playing a role, as a word of the
// moment says, Chance, Always, Not any of them, or, in a loop's Until alone, a fact (a value of
// its type) come to the entity afresh.
func (s Subject) If[W any](what W) Cond {
	switch any(what).(type) {
	case effect.Effect, Role, word, Likelihood, Ever, negated:
		return clause{who: s.who, what: what}
	}
	if reflect.TypeFor[W]().Kind() == reflect.Struct {
		return clause{who: s.who, what: came[W]{}}
	}
	return clause{who: s.who, what: what}
}

// Not is the state what absent, inside If; or, given a Cond, where it does not hold.
func Not(what any) Cond { return negated{what} }

type came[F any] struct{}

func (came[F]) guard(k uint8, w steps.Whose) steps.Guard { return steps.GuardCame[F](k, w) }

type comer interface {
	guard(k uint8, w steps.Whose) steps.Guard
}

// Chance is the state of being drawn with the likelihood p, drawn afresh every step from the
// world's seed, the game time and the entity asked, so a load and a replay draw alike.
func Chance(p float64) Likelihood { return Likelihood{p} }

// Likelihood is what Chance makes.
type Likelihood struct{ p float64 }

// Ever is the state everybody is in.
type Ever struct{}

// Always is the state everybody is in: Self(Always) holds every tick.
var Always Ever

type whoKind uint8

const (
	ofSelf whoKind = iota
	ofOther
	ofAround
	ofEntity
)

type who struct {
	kind  whoKind
	rings int
	of    func() uid.UID64
	bad   string
}

func (w who) String() string {
	switch w.kind {
	case ofSelf:
		return "Self"
	case ofOther:
		return "Other"
	case ofAround:
		return fmt.Sprintf("Around(%d)", w.rings)
	}
	return "On(plugin)"
}

type clause struct {
	who  who
	what any
}

type junction struct {
	all   bool
	parts []Cond
}

type negated struct{ of any }

func (clause) cond()   {}
func (junction) cond() {}
func (negated) cond()  {}

func (c clause) And(o Cond) Cond   { return and(c, o) }
func (c clause) Or(o Cond) Cond    { return or(c, o) }
func (j junction) And(o Cond) Cond { return and(j, o) }
func (j junction) Or(o Cond) Cond  { return or(j, o) }
func (n negated) And(o Cond) Cond  { return and(n, o) }
func (n negated) Or(o Cond) Cond   { return or(n, o) }

func and(a, b Cond) Cond { return join(a, b, true) }
func or(a, b Cond) Cond  { return join(a, b, false) }

func join(a, b Cond, all bool) Cond {
	if j, ok := a.(junction); ok && j.all == all {
		return junction{all: all, parts: append(append([]Cond{}, j.parts...), b)}
	}
	return junction{all: all, parts: []Cond{a, b}}
}

// Lower lays the condition down into a guard for the branch named name; at the top of a rule
// the roles of Self and Other are its host's to narrow by (steps.Lowered).
func (c clause) Lower(as steps.LowerAs, name string) steps.Lowering { return lower(c, as, name) }

// Lower is steps.Lowered.
func (j junction) Lower(as steps.LowerAs, name string) steps.Lowering {
	return lower(j, as, name)
}

// Lower is steps.Lowered.
func (n negated) Lower(as steps.LowerAs, name string) steps.Lowering {
	return lower(n, as, name)
}

func lower(c Cond, as steps.LowerAs, name string) steps.Lowering {
	l := &lowering{name: name, as: as, out: steps.Lowering{OtherRole: -1}}
	l.out.Guard = l.cond(c, as == steps.AtTop)
	return l.out
}

type lowering struct {
	name string
	as   steps.LowerAs
	out  steps.Lowering
}

func (l *lowering) cond(c Cond, top bool) steps.Guard {
	switch v := c.(type) {
	case clause:
		return l.clause(v, top)
	case junction:
		var gs []steps.Guard
		for _, p := range v.parts {
			if g := l.cond(p, top && v.all); g != nil {
				gs = append(gs, g)
			}
		}
		switch {
		case len(gs) == 0:
			return nil
		case len(gs) == 1:
			return gs[0]
		case v.all:
			return steps.GuardAll(gs...)
		}
		return steps.GuardAny(gs...)
	case negated:
		inner, ok := v.of.(Cond)
		if !ok {
			l.fail("Not(%T) says of nobody: write it inside Self, Other, Around or On", v.of)
		}
		return steps.GuardNot(l.cond(inner, false))
	}
	l.fail("%T is no condition", c)
	return nil
}

func (l *lowering) clause(c clause, top bool) steps.Guard {
	if c.who.bad != "" {
		l.fail("%s", c.who.bad)
	}
	return l.state(c.who, l.whose(c.who), c.what, top)
}

func (l *lowering) state(c who, w steps.Whose, what any, top bool) steps.Guard {
	switch v := what.(type) {
	case effect.Effect:
		return steps.GuardUnder(v, w)
	case Role:
		bit := uint8(v.Tag())
		switch {
		case top && c.kind == ofSelf:
			l.out.SelfRoles = append(l.out.SelfRoles, bit)
			return nil
		case top && c.kind == ofOther:
			if l.out.OtherRole >= 0 {
				l.fail("the other's role twice")
			}
			l.out.OtherRole = int(bit)
			return nil
		}
		return steps.GuardPlaying(bit, w)
	case Likelihood:
		return steps.GuardChance(v.p, w)
	case Ever:
		return steps.GuardAlways(w)
	case word:
		if c.kind != ofSelf && c.kind != ofOther {
			l.fail("%s: a word of the moment is of Self or of Other", c)
		}
		if v.ofOther() != (c.kind == ofOther) {
			l.fail("%s(%s): the word is of the %s", c, v.named(), v.side())
		}
		g, of := v.lower(w)
		l.out.Words = append(l.out.Words, of)
		return g
	case negated:
		if _, isCond := v.of.(Cond); isCond {
			l.fail("%s(Not(condition)): Not of a condition stands outside", c)
		}
		switch v.of.(type) {
		case effect.Effect, Role, word, Likelihood, Ever:
		default:
			if reflect.TypeOf(v.of).Kind() == reflect.Struct {
				l.fail("%s(Not(%T{})): a fact comes or has not come yet; no Not of it", c, v.of)
			}
		}
		return steps.GuardNot(l.state(c, w, v.of, false))
	case comer:
		if l.as != steps.AsUntil {
			l.fail("%s(%T{}): a fact that comes is a loop's type, Loop[F], or its Until", c, what)
		}
		if c.kind != ofSelf {
			l.fail("%s(%T{}): a fact comes to the entity: Self alone", c, what)
		}
		if l.out.Came >= steps.MaxCame {
			l.fail("more than %d facts in one Until", steps.MaxCame)
		}
		k := uint8(l.out.Came)
		l.out.Came++
		return v.guard(k, w)
	}
	l.fail("%s(%T): no such state", c, what)
	return nil
}

func (l *lowering) whose(w who) steps.Whose {
	switch w.kind {
	case ofSelf:
		return steps.OfMe
	case ofOther:
		return steps.OfSubject
	case ofAround:
		if l.out.Places > 0 && l.out.Rings != w.rings {
			l.fail("Around(%d) and Around(%d): one ring of places a rule", l.out.Rings, w.rings)
		}
		l.out.Rings, l.out.Places = w.rings, l.out.Places+1
		return steps.OfTarget
	}
	return steps.OfEntity(w.of)
}

func (l *lowering) fail(format string, args ...any) {
	panic(fmt.Sprintf("cond: %q: %s", l.name, fmt.Sprintf(format, args...)))
}
