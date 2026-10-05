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

// eachRule is a rule of a moment of one entity, as rule.Then builds it: what Rules runs over a
// chunk.
type eachRule[P any] interface {
	BindColumns(cols *Columns)
	RunEach(t Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P)
}

// onceRule is a rule over no component, as rule.Then builds one for All: what StepRules runs.
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

// Add takes a rule of P made by rule.Then; ErrUnhosted for another, ErrHostBuilt after Bind.
func (r *Rules[P]) Add(rule any) error {
	each, ok := rule.(eachRule[P])
	if !ok {
		return fmt.Errorf("%w: %v", ErrUnhosted, rule)
	}
	if r.bound {
		return fmt.Errorf("%w: %v", ErrHostBuilt, rule)
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
// entities — the clock's, the weather's. It takes rules over no component (rule.All): a role's
// fire while the entity the moment is About plays the role (Tick.Roles) — the plugin whose entity
// it is plays it.
type StepRules[P any] struct {
	rules []stepRule[P]
	bound bool
}

// stepRule is a rule StepRules runs, with the roles the moment's entity must play for it.
type stepRule[P any] struct {
	once  onceRule[P]
	roles uint64
}

// playedRule is a rule over no component narrowed to roles, a bit each, as a role's Obeys makes it.
type playedRule interface{ OnceRoles() uint64 }

// Add takes a rule of P over no component made by rule.Then, a role's too; ErrUnhosted for
// another, ErrHostBuilt after Bind.
func (r *StepRules[P]) Add(rule any) error {
	once, ok := rule.(onceRule[P])
	if !ok {
		return fmt.Errorf("%w: %v (a rule of the world as a whole takes no filter)", ErrUnhosted, rule)
	}
	if r.bound {
		return fmt.Errorf("%w: %v", ErrHostBuilt, rule)
	}
	s := stepRule[P]{once: once}
	if p, ok := rule.(playedRule); ok {
		s.roles = p.OnceRoles()
	}
	r.rules = append(r.rules, s)
	return nil
}

// Bind marks the system running the rules built: no rule is taken after; call it in its Init.
func (r *StepRules[P]) Bind() { r.bound = true }

// Empty reports whether no rule was added.
func (r *StepRules[P]) Empty() bool { return len(r.rules) == 0 }

// Run runs every rule on about, a role's while the entity about is About plays the role.
func (r *StepRules[P]) Run(t Tick, about P) {
	for _, s := range r.rules {
		if s.roles != 0 && !plays(t, &about, s.roles) {
			continue
		}
		s.once.RunOnce(t, about)
	}
}

// plays reports whether the entity the moment is About plays every one of roles.
func plays(t Tick, moment any, roles uint64) bool {
	a, ok := moment.(About)
	return ok && t.Roles != nil && t.Roles(a.Who())&roles == roles
}
