package rule

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/goke/v3"
)

// eachRunner is a rule with its component type erased.
type eachRunner[P any] interface {
	bind(qb *goke.QueryBuilder, cols columns)
	run(t Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P)
}

// columns are the optional columns a host's query has, by component type: rules over one
// component share its column, the host's own among them.
type columns map[reflect.Type]any

// column is cols' column of T, added to qb at its first.
func column[T any](qb *goke.QueryBuilder, cols columns) *goke.OptComp[T] {
	key := reflect.TypeFor[T]()
	if c, ok := cols[key]; ok {
		return c.(*goke.OptComp[T])
	}
	c := &goke.OptComp[T]{}
	qb.Optional(c)
	cols[key] = c
	return c
}

type every[P any] struct{ react func(Tick, P) }

func (*every[P]) rule() {}

func (e *every[P]) bind(*goke.QueryBuilder, columns) {}

func (e *every[P]) run(t Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P) {
	for i := range cursor.IDs {
		if keep == nil || keep(i) {
			e.react(t, about(i))
		}
	}
}

// EachHost runs the rules made for payload P inside a host's own walk
// over its entities.
type EachHost[P any] struct {
	runners []eachRunner[P]
	own     []func(qb *goke.QueryBuilder, cols columns)
	bound   bool
}

// Own has the host of h read the component T itself, through col — optional in its query, added
// by Bind — shared with the rules over T; call it before Bind.
func Own[T, P any](h *EachHost[P], col *goke.OptComp[T]) {
	h.own = append(h.own, func(qb *goke.QueryBuilder, cols columns) {
		qb.Optional(col)
		cols[reflect.TypeFor[T]()] = col
	})
}

// Empty reports whether no rule was added.
func (h *EachHost[P]) Empty() bool { return len(h.runners) == 0 }

// Add takes a rule for P; ErrUnhosted for another, ErrHostBuilt after Bind.
func (h *EachHost[P]) Add(b Rule) error {
	runner, ok := b.(eachRunner[P])
	if !ok {
		return fmt.Errorf("%w: %T", ErrUnhosted, b)
	}
	if h.bound {
		return fmt.Errorf("%w: %T", ErrHostBuilt, b)
	}
	h.runners = append(h.runners, runner)
	return nil
}

// Bind adds every rule's component to the host's query — call once, before it is built.
func (h *EachHost[P]) Bind(qb *goke.QueryBuilder) {
	h.bound = true
	cols := columns{}
	for _, own := range h.own {
		own(qb, cols)
	}
	for _, r := range h.runners {
		r.bind(qb, cols)
	}
}

// Run runs every rule over the chunk being walked; about(i) describes its i-th entity.
func (h *EachHost[P]) Run(t Tick, cursor *goke.Cursor, about func(i int) P) {
	h.RunWhere(t, cursor, nil, about)
}

// RunWhere is Run over the entities of the chunk keep lets through: a moment only some have.
func (h *EachHost[P]) RunWhere(t Tick, cursor *goke.Cursor, keep func(i int) bool, about func(i int) P) {
	for _, r := range h.runners {
		r.run(t, cursor, keep, about)
	}
}
