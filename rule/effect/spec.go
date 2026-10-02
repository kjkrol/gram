package effect

import (
	"encoding/gob"
	"reflect"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/uid"
)

// Spec is the definition of one effect: what it does to an entity and for how long.
type Spec []Trait

// Trait is one part of a Spec — made with Lasts, Stacking, Grant, Alter or Then.
type Trait interface{ apply(d *def) }

// def is an effect as the plugin runs it.
type def struct {
	name     string
	lasts    time.Duration // 0: Forever
	stacking bool
	grants   []grant
	alters   []alter
	then     effectID // cast once it runs out, when follows
	follows  bool
}

type traitFn func(d *def)

func (f traitFn) apply(d *def) { f(d) }

// Lasts is the default length of every cast of the effect; without it, an effect lasts until
// Dispel. CastFor overrides it per cast.
func Lasts(d time.Duration) Trait { return traitFn(func(e *def) { e.lasts = d }) }

// Stacking makes a repeated Cast add a slot instead of refreshing the one there.
func Stacking() Trait { return traitFn(func(e *def) { e.stacking = true }) }

// Then casts next once the effect runs out — not when it is dispelled: burning, then ash. next is
// one of the same Effects, defined before.
func Then(next Effect) Trait {
	return traitFn(func(e *def) { e.then, e.follows = next.id, true })
}

// Grant gives the entity these tags of family F while the effect runs, and takes them back after
// unless another running effect grants them; an entity without the family gets it attached.
func Grant[F any](tags ...tag.Tag[F]) Trait {
	bits := tag.Tags[F](0).With(tags...)
	return traitFn(func(e *def) {
		e.grants = append(e.grants, grant{
			family: reflect.TypeFor[F](),
			bits:   uint64(bits),
			column: func() column { return &tagColumn[F]{} },
		})
	})
}

// Alter changes the entity's T while the effect runs; the original comes back after. Several
// running Alters of one T stack from the original in slot order, whichever ends first.
func Alter[T any](fn func(v *T)) Trait {
	gob.Register(*new(T))
	return traitFn(func(e *def) {
		e.alters = append(e.alters, alter{comp: reflect.TypeFor[T](), fn: func(v any) { fn(v.(*T)) },
			column: func() column { return &valueColumn[T]{} }})
	})
}

// grant is one Grant with its family erased.
type grant struct {
	family reflect.Type
	bits   uint64
	column func() column
}

// alter is one Alter with its component erased.
type alter struct {
	comp   reflect.Type
	fn     func(v any)
	column func() column
}

// column is one component type the system reads and writes on its query: a tag family or an
// altered component.
type column interface {
	bind(si *goke.SysInit, qb *goke.QueryBuilder)
	present(cursor *goke.Cursor) bool
}

// tagColumn reads and writes one family's bits, and attaches the family where it is missing.
type tagColumn[F any] struct {
	comp goke.OptComp[tag.Tags[F]]
	id   goke.CompID
}

func (c *tagColumn[F]) bind(si *goke.SysInit, qb *goke.QueryBuilder) {
	c.id = si.RegComp[tag.Tags[F]]()
	qb.Optional(&c.comp)
}
func (c *tagColumn[F]) present(cursor *goke.Cursor) bool { return c.comp.Present(cursor) }
func (c *tagColumn[F]) attach(cb *goke.CmdBuf, id uid.UID64, bits uint64) {
	cb.AddOne(id, c.id, tag.Tags[F](bits))
}
func (c *tagColumn[F]) or(cursor *goke.Cursor, i int, bits uint64) {
	c.comp.Slice(cursor)[i] |= tag.Tags[F](bits)
}
func (c *tagColumn[F]) clear(cursor *goke.Cursor, i int, bits uint64) {
	c.comp.Slice(cursor)[i] &^= tag.Tags[F](bits)
}

// valueColumn reads and writes one altered component type.
type valueColumn[T any] struct{ comp goke.OptComp[T] }

func (c *valueColumn[T]) bind(_ *goke.SysInit, qb *goke.QueryBuilder) { qb.Optional(&c.comp) }
func (c *valueColumn[T]) present(cursor *goke.Cursor) bool            { return c.comp.Present(cursor) }
func (c *valueColumn[T]) read(cursor *goke.Cursor, i int) any         { return c.comp.Slice(cursor)[i] }
func (c *valueColumn[T]) write(cursor *goke.Cursor, i int, v any)     { c.comp.Slice(cursor)[i] = v.(T) }
func (c *valueColumn[T]) at(cursor *goke.Cursor, i int) any           { return &c.comp.Slice(cursor)[i] }
