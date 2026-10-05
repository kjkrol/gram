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

// Rule is what is done at a moment a plugin catches in its own pass over its entities — a unit
// standing on the board, one seeing another, two striking — built with Then and hooked through the
// Stage's Initializer (game.Initializer.Hook). The moment's type says which plugin hosts it;
// another refuses it. String is its name and its moment.
type Rule interface {
	fmt.Stringer
	rule()
	narrowed(n narrowing) Rule
}

// within is r for the entities carrying t alone — on a moment that is Met, for the pairs whose
// entity carries t — whatever r's own filter: what a Role's Obeys makes of a rule; desc says how,
// in its String. Narrowed by tag.Any, r is r.
func within[F any](t tag.Tag[F], r Rule, desc string) Rule {
	if reflect.TypeFor[F]() == reflect.TypeFor[tag.Anything]() {
		return r
	}
	return r.narrowed(narrowing{side: plugin.SideOf(t), carrier: carrierOf(t), desc: desc})
}

// Filter is whom a rule fires for: All, Self, Between or Having.
type Filter struct {
	self, other plugin.Side
	paired      bool  // Self or Between
	others      bool  // Between
	mine        *side // Self, for a moment that is not Met
	need        *having
}

// All lets every entity the plugin shows the rule through — every pair, for a moment that is Met.
var All Filter

// Self lets through an entity carrying t.
func Self[F any](t tag.Tag[F]) Filter {
	return Filter{self: plugin.SideOf(t), paired: true, mine: carrierOf(t)}
}

// Between lets through a pair whose entity carries a and whose other carries b — tag.Any for
// either — for a moment that is Met.
func Between[FA, FB any](a tag.Tag[FA], b tag.Tag[FB]) Filter {
	return Filter{self: plugin.SideOf(a), other: plugin.SideOf(b), paired: true, others: true}
}

// Other lets through a pair whose other plays role, whoever its entity is — whom a predator
// sees, when the role is prey — for a moment that is Met; obeyed by a role, the pairs of its
// players with the other role's.
func Other(role *Part) Filter { return Between(tag.Any, role.tag) }

// Having lets through an entity carrying the component T.
func Having[T any]() Filter {
	return Filter{need: &having{state: func() state { return stateOf[T]() }}}
}

// side is a Self filter; on a moment that is not Met it reads the entity's tags as the rule's
// state.
type side struct {
	tags    func() state
	carries func(state any) bool
}

// carrierOf is the side of the entities carrying t.
func carrierOf[F any](t tag.Tag[F]) *side {
	return &side{tags: func() state { return stateOf[tag.Tags[F]]() },
		carries: func(state any) bool { return state.(*tag.Tags[F]).Has(t) }}
}

// cond is a test of the side, as a rule over one entity reads it: the entity's tags carrying a tag.
func (s *side) cond() cond { return cond{mk: s.tags, holds: s.carries} }

// narrowing is a tag a rule's entity must carry besides its filter (a role's Obeys): read off its
// tags by a rule over one entity, matched as one more side by a rule over pairs.
type narrowing struct {
	side    plugin.Side
	carrier *side
	desc    string // what the narrowed rule's String adds
}

// label is the narrowed rule's String: of a rule labelled l.
func (n narrowing) label(l string) string { return l + ", " + n.desc }

// having is a Having filter.
type having struct{ state func() state }

// fired is a rule's body, run on each moment its plugin hands it.
type fired[P any] struct {
	run     *steps.Instant
	current P
	about   bool // P is About an entity
	subject bool // P names a Subject, whom an Aimed command is about
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

// build is the rule named name over root, for whom filter lets through, as its plugin's
// rule-driven system takes it.
func build[P any](name string, filter Filter, root Step) Rule {
	label := fmt.Sprintf("%q of %v", name, reflect.TypeFor[P]())
	f := &fired[P]{}
	f.run = steps.NewInstant(name, root, &f.current)
	_, f.about = any(&f.current).(plugin.About)
	_, f.subject = any(&f.current).(plugin.Subject)
	_, met := any(*new(P)).(plugin.Met)
	react := func(t plugin.Tick, about P) { f.fire(t, nil, about) }
	switch {
	case filter.need != nil:
		return newEachWith(label, react, cond{mk: filter.need.state})
	case filter.paired && !met:
		if filter.mine == nil || filter.others {
			panic(fmt.Sprintf("rule: %q: Between needs a moment with others (Met)", name))
		}
		return newEachWith(label, react, filter.mine.cond())
	case filter.paired || met:
		return &pair[P]{label: label, self: filter.self, other: filter.other, react: react}
	}
	return &every[P]{label: label, react: react}
}

func (e *every[P]) narrowed(n narrowing) Rule {
	return newEachWith(n.label(e.label), e.react, n.carrier.cond())
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
