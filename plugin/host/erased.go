package host

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
)

// Side is one side of a pair, its tag family erased: what a rule built from separate filters
// hands PairOf.
type Side struct{ t tagged }

// SideOf is t as a side of a pair.
func SideOf[F any](t tag.Tag[F]) Side { return Side{tagOf(t)} }

// AnySide takes whatever is on that side.
var AnySide Side

// PairOf is Pair over sides already erased: a rule for every pair a host meets where one
// entity is a and the other b.
func PairOf[P any](a, b Side, react func(plugin.Tick, P)) plugin.Rule {
	return &pair[P]{a: a.t, b: b.t, same: a.t.family == b.t.family && a.t.bit == b.t.bit, react: react}
}

// State is a component a rule reads, its type erased: made by StateOf, bound to one
// host's query.
type State interface {
	bind(qb *goke.QueryBuilder, cols columns)
	present(cursor *goke.Cursor) bool
	at(cursor *goke.Cursor, i int) any
}

// StateOf is the component T as a rule's state; the rule is handed a *T.
func StateOf[T any]() State { return &stateOf[T]{} }

type stateOf[T any] struct{ comp *goke.OptComp[T] }

func (s *stateOf[T]) bind(qb *goke.QueryBuilder, cols columns) { s.comp = column[T](qb, cols) }
func (s *stateOf[T]) present(cursor *goke.Cursor) bool         { return s.comp.Present(cursor) }
func (s *stateOf[T]) at(cursor *goke.Cursor, i int) any        { return &s.comp.Slice(cursor)[i] }

// EachWith is Each over a state already erased: run on every entity a host visits that carries
// the state's component, handed a pointer to it.
func EachWith[P any](s State, react func(t plugin.Tick, state any, about P)) plugin.Rule {
	return &eachWith[P]{s: s, react: react}
}

type eachWith[P any] struct {
	s     State
	react func(plugin.Tick, any, P)
}

func (e *eachWith[P]) bind(qb *goke.QueryBuilder, cols columns) { e.s.bind(qb, cols) }

func (e *eachWith[P]) run(t plugin.Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P) {
	if !e.s.present(cursor) {
		return
	}
	for i := range cursor.IDs {
		if keep == nil || keep(i) {
			e.react(t, e.s.at(cursor, i), about(i))
		}
	}
}

// ListHost runs the Every rules made for payload P once a pass, walking no entities: for a
// moment of the world as a whole — its clock's.
type ListHost[P any] struct{ reacts []func(plugin.Tick, P) }

// Add takes a rule for P over no state; plugin.ErrUnhosted for another.
func (h *ListHost[P]) Add(b plugin.Rule) error {
	e, ok := b.(*every[P])
	if !ok {
		return fmt.Errorf("%w: %T", plugin.ErrUnhosted, b)
	}
	h.reacts = append(h.reacts, e.react)
	return nil
}

// Empty reports whether no rule was added.
func (h *ListHost[P]) Empty() bool { return len(h.reacts) == 0 }

// Run runs every rule on about.
func (h *ListHost[P]) Run(t plugin.Tick, about P) {
	for _, r := range h.reacts {
		r(t, about)
	}
}
