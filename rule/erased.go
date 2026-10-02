package rule

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
)

// pairSide is one side of a pair, its tag family erased: what a rule built from separate filters
// hands pairOf.
type pairSide struct{ t tagged }

// pairSideOf is t as a side of a pair.
func pairSideOf[F any](t tag.Tag[F]) pairSide { return pairSide{tagOf(t)} }

// anyPairSide takes whatever is on that side.
var anyPairSide pairSide

// pairOf is a rule for every pair a host meets where one entity is a and the other b.
func pairOf[P any](a, b pairSide, react func(Tick, P)) Rule {
	return &pair[P]{a: a.t, b: b.t, same: a.t.family == b.t.family && a.t.bit == b.t.bit, react: react}
}

// ruleState is a component a rule reads, its type erased: made by stateOf, bound to one
// host's query.
type ruleState interface {
	bind(qb *goke.QueryBuilder, cols columns)
	present(cursor *goke.Cursor) bool
	at(cursor *goke.Cursor, i int) any
}

// stateOf is the component T as a rule's state; the rule is handed a *T.
func stateOf[T any]() ruleState { return &stateCol[T]{} }

type stateCol[T any] struct{ comp *goke.OptComp[T] }

func (s *stateCol[T]) bind(qb *goke.QueryBuilder, cols columns) { s.comp = column[T](qb, cols) }
func (s *stateCol[T]) present(cursor *goke.Cursor) bool         { return s.comp.Present(cursor) }
func (s *stateCol[T]) at(cursor *goke.Cursor, i int) any        { return &s.comp.Slice(cursor)[i] }

// newEachWith is a rule run on every entity a host visits that carries the state's component,
// handed a pointer to it.
func newEachWith[P any](s ruleState, react func(t Tick, state any, about P)) Rule {
	return &eachWith[P]{s: s, react: react}
}

type eachWith[P any] struct {
	s     ruleState
	react func(Tick, any, P)
}

func (*eachWith[P]) rule() {}

func (e *eachWith[P]) bind(qb *goke.QueryBuilder, cols columns) { e.s.bind(qb, cols) }

func (e *eachWith[P]) run(t Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P) {
	if !e.s.present(cursor) {
		return
	}
	for i := range cursor.IDs {
		if keep == nil || keep(i) {
			e.react(t, e.s.at(cursor, i), about(i))
		}
	}
}

// ListHost runs the rules made for payload P over no state (rule.All) once a pass, walking no
// entities: for a moment of the world as a whole — its clock's.
type ListHost[P any] struct{ reacts []func(Tick, P) }

// Add takes a rule for P over no state; ErrUnhosted for another.
func (h *ListHost[P]) Add(b Rule) error {
	e, ok := b.(*every[P])
	if !ok {
		return fmt.Errorf("%w: %T", ErrUnhosted, b)
	}
	h.reacts = append(h.reacts, e.react)
	return nil
}

// Empty reports whether no rule was added.
func (h *ListHost[P]) Empty() bool { return len(h.reacts) == 0 }

// Run runs every rule on about.
func (h *ListHost[P]) Run(t Tick, about P) {
	for _, r := range h.reacts {
		r(t, about)
	}
}
