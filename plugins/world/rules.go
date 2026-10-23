package world

import (
	"errors"
	"reflect"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
)

// Rule is how a renderer draws the entities it draws, settled for each of them every frame, in
// the order given. A rule reads one component of the entity, T, and holds only for one carrying it.
type Rule interface {
	bind(qb *goke.QueryBuilder, cols columns)
	run(cur *goke.Cursor, layers [][]render.Appearance, shown []bool)
	shows() bool
}

// Over draws with on top of every entity carrying T, where each of when holds.
func Over[T any](with render.Appearance, when ...func(T) bool) Rule {
	return &dressing[T]{when: when, dress: func(l *[]render.Appearance, _ T) { *l = append(*l, with) }}
}

// As draws every entity carrying T as with, in place of its own sprite, where each of when holds.
func As[T any](with render.Appearance, when ...func(T) bool) Rule {
	return &dressing[T]{when: when, dress: func(l *[]render.Appearance, _ T) { (*l)[0] = with }}
}

// Swap draws every entity carrying T, where each of when holds, as the twin of the sprite it
// would be drawn with; one with no twin as it is.
func Swap[T any](twins map[render.SpriteID]render.SpriteID, when ...func(T) bool) Rule {
	return &dressing[T]{when: when, dress: func(l *[]render.Appearance, _ T) {
		if id, ok := twins[(*l)[0].SpriteID]; ok {
			(*l)[0].SpriteID = id
		}
	}}
}

// With reworks the sprite of every entity carrying T through fn, which sees the T it carries,
// where each of when holds.
func With[T any](fn func(render.Appearance, T) render.Appearance, when ...func(T) bool) Rule {
	return &dressing[T]{when: when, dress: func(l *[]render.Appearance, t T) { (*l)[0] = fn((*l)[0], t) }}
}

// Show draws every entity carrying T where each of when holds. Once a Show rule is given, a
// renderer draws only those some Show rule holds for.
func Show[T any](when ...func(T) bool) Rule { return &dressing[T]{when: when} }

type dressing[T any] struct {
	col   func(*goke.Cursor) []T
	when  []func(T) bool
	dress func(l *[]render.Appearance, t T)
}

func (d *dressing[T]) bind(qb *goke.QueryBuilder, cols columns) { d.col = column[T](qb, cols) }

func (d *dressing[T]) shows() bool { return d.dress == nil }

func (d *dressing[T]) run(cur *goke.Cursor, layers [][]render.Appearance, shown []bool) {
	ts := d.col(cur)
	for i := range ts {
		if !d.holds(ts[i]) {
			continue
		}
		switch {
		case d.dress == nil && shown != nil:
			shown[i] = true
		case d.dress != nil && layers != nil:
			d.dress(&layers[i], ts[i])
		}
	}
}

func (d *dressing[T]) holds(t T) bool {
	for _, w := range d.when {
		if !w(t) {
			return false
		}
	}
	return true
}

// rules over one component share its column, the renderer's own among them
type columns map[reflect.Type]any

func column[T any](qb *goke.QueryBuilder, cols columns) func(*goke.Cursor) []T {
	key := reflect.TypeFor[T]()
	if c, ok := cols[key]; ok {
		return c.(func(*goke.Cursor) []T)
	}
	c := &goke.OptComp[T]{}
	qb.Optional(c)
	slice := c.Slice
	cols[key] = slice
	return slice
}

// Rules are the rules a renderer runs over the chunks of entities it draws.
type Rules struct {
	rules []Rule
	own   []func(cols columns)
	shows bool
	bound bool
}

// Add takes rules, run after those taken before. It is an error once the renderer is built.
func (r *Rules) Add(rules ...Rule) error {
	if r.bound {
		return errors.New("world: rules added after the renderer was built")
	}
	for _, rule := range rules {
		r.shows = r.shows || rule.shows()
	}
	r.rules = append(r.rules, rules...)
	return nil
}

// Own has the rules over T read the column col of the renderer's own query. Call it before Bind.
func Own[T any](r *Rules, col interface{ Slice(*goke.Cursor) []T }) {
	r.own = append(r.own, func(cols columns) { cols[reflect.TypeFor[T]()] = col.Slice })
}

// Bind adds the components the rules read to the renderer's query. Call it once, before it is
// built.
func (r *Rules) Bind(qb *goke.QueryBuilder) {
	r.bound = true
	cols := columns{}
	for _, own := range r.own {
		own(cols)
	}
	for _, rule := range r.rules {
		rule.bind(qb, cols)
	}
}

// Run settles the chunk under cur: layers[i] begins as its i-th entity's render.Appearance and ends as
// what it is drawn with, shown[i] whether it is drawn; either may be nil.
func (r *Rules) Run(cur *goke.Cursor, layers [][]render.Appearance, shown []bool) {
	for i := range shown {
		shown[i] = !r.shows
	}
	for _, rule := range r.rules {
		rule.run(cur, layers, shown)
	}
}
