// Package comp is how a kind's Spec names one component and where its value comes from: the same
// for every entity of the kind (Const), read from each entity's own row (Load), the tags of one
// family (Tagged), or a default of the world's roster left out (Without). Nothing here is
// implemented by a game; the world spawns through a Comp's Spawner.
package comp

import (
	"reflect"
	"unsafe"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/uid"
)

// Comp is one component of a kind's Spec — made with Const, Load, Tagged or Without, never
// implemented by a game. Its methods are the spawning world's way in.
type Comp interface {
	Spawner() Spawner
	LoadToken() goke.CompToken
	rowType() reflect.Type
	compType() reflect.Type
}

// Spawner writes one component onto the entities a world spawns — what a world asks a Comp for.
type Spawner interface {
	Columns() []goke.Addable
	Write(cursor *goke.Cursor, i int, row any, id uid.UID64)
}

// TypeOf is the component type c stands for.
func TypeOf(c Comp) reflect.Type { return c.compType() }

// RowOf is the row type a Load reads, nil for a component that reads none.
func RowOf(c Comp) reflect.Type { return c.rowType() }

// Template yields one component value per spawned entity, optionally running
// an effect right after it is written.
type Template[T any] struct {
	value  func(row any) T
	effect func(v T, id uid.UID64)
	row    reflect.Type
}

var _ Comp = Template[struct{}]{}

// Const is a component whose value is the same for every entity of the kind.
func Const[T any](v T) Template[T] {
	return Template[T]{value: func(any) T { return v }}
}

// Load is a component whose value is read from each entity's own row.
func Load[P, T any](load func(row P) T) Template[T] {
	return Template[T]{value: func(row any) T { return load(row.(P)) }, row: reflect.TypeFor[P]()}
}

// WithEffect sets a callback run right after this component is written for each spawned entity.
func (t Template[T]) WithEffect(effect func(v T, id uid.UID64)) Template[T] {
	t.effect = effect
	return t
}

// Resolve is the value for one entity, with its effect run.
func (t Template[T]) Resolve(row any, id uid.UID64) T {
	v := t.value(row)
	if t.effect != nil {
		t.effect(v, id)
	}
	return v
}

// Spawner is this component's column and the writing of it — see Comp.
func (t Template[T]) Spawner() Spawner { return &writer[T]{template: t} }

// LoadToken names T to a save file being loaded — see Comp.
func (t Template[T]) LoadToken() goke.CompToken { return goke.LoadComp[T]() }

func (t Template[T]) rowType() reflect.Type  { return t.row }
func (t Template[T]) compType() reflect.Type { return reflect.TypeFor[T]() }

type writer[T any] struct {
	comp     goke.Comp[T]
	template Template[T]
}

func (w *writer[T]) Columns() []goke.Addable { return []goke.Addable{&w.comp} }

func (w *writer[T]) Write(cursor *goke.Cursor, i int, row any, id uid.UID64) {
	v := w.template.Resolve(row, id)
	if unsafe.Sizeof(v) > 0 {
		w.comp.Slice(cursor)[i] = v
	}
}
