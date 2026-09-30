package host

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
)

// eachRunner is a trigger with its component type erased.
type eachRunner[P any] interface {
	bind(qb *goke.QueryBuilder, cols columns)
	run(t plugin.Tick, cursor *goke.Cursor, rows []int, about func(i int) P)
}

// columns are the optional columns a host's query has, by component type: triggers over one
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

type each[T, P any] struct {
	state *goke.OptComp[T]
	react func(plugin.Tick, *T, P)
}

func (e *each[T, P]) bind(qb *goke.QueryBuilder, cols columns) { e.state = column[T](qb, cols) }

func (e *each[T, P]) run(t plugin.Tick, cursor *goke.Cursor, rows []int, about func(i int) P) {
	if !e.state.Present(cursor) {
		return
	}
	states := e.state.Slice(cursor)
	if rows != nil {
		for _, i := range rows {
			e.react(t, &states[i], about(i))
		}
		return
	}
	for i := range cursor.IDs {
		e.react(t, &states[i], about(i))
	}
}

type every[P any] struct{ react func(plugin.Tick, P) }

func (e *every[P]) bind(*goke.QueryBuilder, columns) {}

func (e *every[P]) run(t plugin.Tick, cursor *goke.Cursor, rows []int, about func(i int) P) {
	if rows != nil {
		for _, i := range rows {
			e.react(t, about(i))
		}
		return
	}
	for i := range cursor.IDs {
		e.react(t, about(i))
	}
}

// EachHost runs the triggers made for payload P inside a host's own walk
// over its entities.
type EachHost[P any] struct {
	runners []eachRunner[P]
	own     []func(qb *goke.QueryBuilder, cols columns)
	bound   bool
}

// Own has the host of h read the component T itself, through col — optional in its query, added
// by Bind — shared with the triggers over T; call it before Bind.
func Own[T, P any](h *EachHost[P], col *goke.OptComp[T]) {
	h.own = append(h.own, func(qb *goke.QueryBuilder, cols columns) {
		qb.Optional(col)
		cols[reflect.TypeFor[T]()] = col
	})
}

// Empty reports whether no trigger was added.
func (h *EachHost[P]) Empty() bool { return len(h.runners) == 0 }

// Add takes a trigger for P; plugin.ErrUnhosted for another, plugin.ErrHostBuilt after Bind.
func (h *EachHost[P]) Add(b plugin.Trigger) error {
	runner, ok := b.(eachRunner[P])
	if !ok {
		return fmt.Errorf("%w: %T", plugin.ErrUnhosted, b)
	}
	if h.bound {
		return fmt.Errorf("%w: %T", plugin.ErrHostBuilt, b)
	}
	h.runners = append(h.runners, runner)
	return nil
}

// Bind adds every trigger's component to the host's query — call once, before it is built.
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

// Run runs every trigger over the chunk being walked; about(i) describes its i-th entity.
func (h *EachHost[P]) Run(t plugin.Tick, cursor *goke.Cursor, about func(i int) P) {
	for _, r := range h.runners {
		r.run(t, cursor, nil, about)
	}
}

// RunRows is Run over the rows of the chunk given alone: those a marker picks out.
func (h *EachHost[P]) RunRows(t plugin.Tick, cursor *goke.Cursor, rows []int, about func(i int) P) {
	if len(rows) == 0 {
		return
	}
	for _, r := range h.runners {
		r.run(t, cursor, rows, about)
	}
}
