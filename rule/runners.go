package rule

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
)

// The rules On builds, as the rule-driven systems of package plugin run them: every over every
// entity (plugin.Rules) or once a step (plugin.StepRules), eachWith over those carrying a
// component, pair over pairs (plugin.PairRules).

type every[P any] struct{ react func(plugin.Tick, P) }

func (*every[P]) rule() {}

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

type eachWith[P any] struct {
	s     state
	react func(plugin.Tick, any, P)
}

func (*eachWith[P]) rule() {}

func (e *eachWith[P]) BindColumns(cols *plugin.Columns) { e.s.bind(cols) }

func (e *eachWith[P]) RunEach(t plugin.Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P) {
	if !e.s.present(cursor) {
		return
	}
	for i := range cursor.IDs {
		if keep == nil || keep(i) {
			e.react(t, e.s.at(cursor, i), about(i))
		}
	}
}

type pair[P any] struct {
	self, other plugin.Side
	react       func(plugin.Tick, P)
}

func (*pair[P]) rule() {}

func (p *pair[P]) PairSides() (self, other plugin.Side) { return p.self, p.other }

func (p *pair[P]) RunPair(t plugin.Tick, about P) { p.react(t, about) }
