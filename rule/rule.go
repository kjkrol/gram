package rule

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/uid"
)

// Rule is what a role's players do at a moment a plugin catches: a Branch of that moment, or a
// role whose rules come with it. The moment's type says which plugin hosts it.
type Rule interface {
	fmt.Stringer
	rule()
	narrowed(n narrowing) Rule
}

func within[F any](t tag.Tag[F], r Rule, desc string) Rule {
	if reflect.TypeFor[F]() == reflect.TypeFor[tag.Anything]() {
		return r
	}
	n := narrowing{side: plugin.SideOf(t), carrier: carrierOf(t), desc: desc}
	if role, ok := any(t).(tag.Tag[Roles]); ok {
		n.role = 1 << role
	}
	return r.narrowed(n)
}

type side struct {
	tags    func() state
	carries func(state any) bool
}

func carrierOf[F any](t tag.Tag[F]) *side {
	return &side{tags: func() state { return stateOf[tag.Tags[F]]() },
		carries: func(state any) bool { return state.(*tag.Tags[F]).Has(t) }}
}

func (s *side) cond() cond { return cond{mk: s.tags, holds: s.carries} }

type narrowing struct {
	side    plugin.Side
	carrier *side
	desc    string
	role    uint64
}

func (n narrowing) label(l string) string {
	if n.desc == "" {
		return l
	}
	return l + ", " + n.desc
}

type fired[P any] struct {
	run     *steps.Instant
	current P
	about   bool
	subject bool
}

func (f *fired[P]) fire(t plugin.Tick, _ any, about P) {
	f.current = about
	var id, subject uid.UID64
	var aimed bool
	if f.about {
		id = any(&f.current).(plugin.About).Who() // a pointer: no copy to the heap
	}
	if f.subject {
		subject, aimed = any(&f.current).(plugin.Subject).Subject()
	}
	p := steps.Pass{Commands: t.Commands, Effects: t.Effects, Dt: t.Dt, Time: t.Time, Seed: t.Seed, World: t.World, Around: t.Around, Roles: t.Roles}
	f.run.Fire(p, t.CmdBuf, id, f.about, subject, aimed)
}

func construct[P any](name string, filter sides, root steps.Step) Rule {
	label := fmt.Sprintf("%q of %v", name, reflect.TypeFor[P]())
	f := &fired[P]{}
	f.run = steps.NewInstant(name, root, &f.current)
	_, f.about = any(&f.current).(plugin.About)
	_, f.subject = any(&f.current).(plugin.Subject)
	_, met := any(*new(P)).(plugin.Met)
	react := func(t plugin.Tick, about P) { f.fire(t, nil, about) }
	switch {
	case filter.paired && !met:
		panic(fmt.Sprintf("rule: %q: the other's role needs a moment with others (Met)", name))
	case met:
		return &pair[P]{label: label, self: filter.self, other: filter.other, react: react}
	}
	return &every[P]{label: label, react: react}
}
func (e *every[P]) narrowed(n narrowing) Rule {
	each := newEachWith(n.label(e.label), e.react, n.carrier.cond())
	if n.role == 0 {
		return each
	}
	return &played[P]{eachWith: each, roles: n.role}
}

func (p *played[P]) narrowed(n narrowing) Rule {
	each := p.eachWith.narrowed(n).(*eachWith[P])
	if n.role == 0 {
		return each
	}
	return &played[P]{eachWith: each, roles: p.roles | n.role}
}

func (e *eachWith[P]) narrowed(n narrowing) Rule {
	conds := make([]cond, 0, len(e.conds)+1)
	for _, c := range e.conds {
		conds = append(conds, cond{mk: c.mk, holds: c.holds})
	}
	return newEachWith(n.label(e.label), e.react, append(conds, n.carrier.cond())...)
}

func (p *pair[P]) narrowed(n narrowing) Rule {
	q := *p
	q.label = n.label(p.label)
	q.within = append(slices.Clone(p.within), n.side)
	return &q
}
