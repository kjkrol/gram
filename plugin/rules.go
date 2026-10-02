package plugin

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/goke/v3"
)

// Columns are the optional columns of a rule-driven system's query, by component: the rules over
// one component — and the system's own reading of it (Own) — share one.
type Columns struct {
	qb   *goke.QueryBuilder
	cols map[reflect.Type]any
}

// Of is the column of T, made optional in the query at its first.
func (c *Columns) Of[T any]() *goke.OptComp[T] {
	key := reflect.TypeFor[T]()
	if col, ok := c.cols[key]; ok {
		return col.(*goke.OptComp[T])
	}
	col := &goke.OptComp[T]{}
	c.qb.Optional(col)
	c.cols[key] = col
	return col
}

// eachRule is a rule of a moment of one entity, as rule.On builds it: what Rules runs over a
// chunk.
type eachRule[P any] interface {
	BindColumns(cols *Columns)
	RunEach(t Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P)
}

// onceRule is a rule over no component, as rule.On builds one for All: what StepRules runs.
type onceRule[P any] interface {
	RunOnce(t Tick, about P)
}

// Rules are the rules of a moment P a rule-driven system runs in the pass over its entities it
// makes anyway — a Moving, a unit.Standing: Bind them into its query in Init, Run them over each
// chunk in Update. One walk runs them all.
type Rules[P any] struct {
	rules []eachRule[P]
	own   []func(cols *Columns)
	bound bool
}

// Own has the system read the component T itself through col, shared with the rules over T —
// optional in its query, added by Bind; call it before Bind.
func Own[T, P any](r *Rules[P], col *goke.OptComp[T]) {
	r.own = append(r.own, func(cols *Columns) {
		cols.qb.Optional(col)
		cols.cols[reflect.TypeFor[T]()] = col
	})
}

// Empty reports whether no rule was added.
func (r *Rules[P]) Empty() bool { return len(r.rules) == 0 }

// Add takes a rule of P made by rule.On; ErrUnhosted for another, ErrHostBuilt after Bind.
func (r *Rules[P]) Add(rule any) error {
	each, ok := rule.(eachRule[P])
	if !ok {
		return fmt.Errorf("%w: %T", ErrUnhosted, rule)
	}
	if r.bound {
		return fmt.Errorf("%w: %T", ErrHostBuilt, rule)
	}
	r.rules = append(r.rules, each)
	return nil
}

// Bind adds what the rules read to the system's query; call once, before it is built.
func (r *Rules[P]) Bind(qb *goke.QueryBuilder) {
	r.bound = true
	cols := &Columns{qb: qb, cols: map[reflect.Type]any{}}
	for _, own := range r.own {
		own(cols)
	}
	for _, each := range r.rules {
		each.BindColumns(cols)
	}
}

// Run runs every rule over the chunk being walked; about(i) is the moment of its i-th entity.
func (r *Rules[P]) Run(t Tick, cursor *goke.Cursor, about func(i int) P) {
	r.RunWhere(t, cursor, nil, about)
}

// RunWhere is Run over the entities of the chunk keep lets through: a moment only some have.
func (r *Rules[P]) RunWhere(t Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P) {
	for _, each := range r.rules {
		each.RunEach(t, cursor, keep, about)
	}
}

// StepRules are the rules of a moment P of the world as a whole, run once a step and walking no
// entities — the clock's. Only rules over no component (rule.All) are taken.
type StepRules[P any] struct{ rules []onceRule[P] }

// Add takes a rule of P over no component made by rule.On; ErrUnhosted for another.
func (r *StepRules[P]) Add(rule any) error {
	once, ok := rule.(onceRule[P])
	if !ok {
		return fmt.Errorf("%w: %T", ErrUnhosted, rule)
	}
	r.rules = append(r.rules, once)
	return nil
}

// Empty reports whether no rule was added.
func (r *StepRules[P]) Empty() bool { return len(r.rules) == 0 }

// Run runs every rule on about.
func (r *StepRules[P]) Run(t Tick, about P) {
	for _, once := range r.rules {
		once.RunOnce(t, about)
	}
}
