package rule

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
)

// The rules On builds, as the rule-driven systems of package plugin run them: every over every
// entity (plugin.Rules) or once a step (plugin.StepRules), eachWith over those carrying a
// component, pair over pairs (plugin.PairRules).

type every[P any] struct {
	label string
	react func(plugin.Tick, P)
}

func (*every[P]) rule() {}

func (e *every[P]) String() string { return e.label }

func (*every[P]) BindColumns(*plugin.Columns) {}

func (e *every[P]) RunEach(t plugin.Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P) {
	for i := range cursor.IDs {
		if keep == nil || keep(i) {
			e.react(t, about(i))
		}
	}
}

func (e *every[P]) RunOnce(t plugin.Tick, about P) { e.react(t, about) }

// state is a component a rule reads, its type erased: made by stateOf, bound to one system's
// query.
type state interface {
	bind(cols *plugin.Columns)
	present(cursor *goke.Cursor) bool
	at(cursor *goke.Cursor, i int) any
}

// stateOf is the component T as a rule's state; the rule is handed a *T.
func stateOf[T any]() state { return &stateCol[T]{} }

type stateCol[T any] struct{ comp *goke.OptComp[T] }

func (s *stateCol[T]) bind(cols *plugin.Columns)         { s.comp = cols.Of[T]() }
func (s *stateCol[T]) present(cursor *goke.Cursor) bool  { return s.comp.Present(cursor) }
func (s *stateCol[T]) at(cursor *goke.Cursor, i int) any { return &s.comp.Slice(cursor)[i] }

// cond is a component an entity must carry for a rule over one entity to fire for it, and what
// must hold of it (nil: carrying it is enough).
type cond struct {
	mk    func() state
	s     state
	holds func(state any) bool
}

type eachWith[P any] struct {
	label string
	conds []cond
	react func(plugin.Tick, P)
}

// newEachWith is the rule over the entities meeting every cond, each with a state of its own.
func newEachWith[P any](label string, react func(plugin.Tick, P), conds ...cond) *eachWith[P] {
	for i := range conds {
		conds[i].s = conds[i].mk()
	}
	return &eachWith[P]{label: label, conds: conds, react: react}
}

func (*eachWith[P]) rule() {}

func (e *eachWith[P]) String() string { return e.label }

func (e *eachWith[P]) BindColumns(cols *plugin.Columns) {
	for _, c := range e.conds {
		c.s.bind(cols)
	}
}

func (e *eachWith[P]) RunEach(t plugin.Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P) {
	for _, c := range e.conds {
		if !c.s.present(cursor) {
			return
		}
	}
	for i := range cursor.IDs {
		if (keep == nil || keep(i)) && e.holds(cursor, i) {
			e.react(t, about(i))
		}
	}
}

// holds reports whether the i-th entity of the chunk meets every cond.
func (e *eachWith[P]) holds(cursor *goke.Cursor, i int) bool {
	for _, c := range e.conds {
		if c.holds != nil && !c.holds(c.s.at(cursor, i)) {
			return false
		}
	}
	return true
}

type pair[P any] struct {
	label       string
	self, other plugin.Side
	within      []plugin.Side // more tags the pair's entity carries (Within)
	react       func(plugin.Tick, P)
}

func (*pair[P]) rule() {}

func (p *pair[P]) String() string { return p.label }

func (p *pair[P]) PairSides() (self, other plugin.Side) { return p.self, p.other }

func (p *pair[P]) PairWithin() []plugin.Side { return p.within }

func (p *pair[P]) RunPair(t plugin.Tick, about P) { p.react(t, about) }
