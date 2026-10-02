package comp

import (
	"reflect"

	"github.com/kjkrol/goke/v3"
)

// Without leaves the roster's default T out of a kind's Spec — a unit nothing pushes leaves out
// collision.Physics. It is a marker kind.Role.Spec takes out; it never reaches a spawn.
func Without[T any]() Comp { return omit{reflect.TypeFor[T]()} }

// Omitted reports whether c is a Without marker, and of which type.
func Omitted(c Comp) (reflect.Type, bool) {
	o, ok := c.(omit)
	return o.t, ok
}

type omit struct{ t reflect.Type }

func (omit) Spawner() Spawner          { return nil }
func (omit) LoadToken() goke.CompToken { return goke.CompToken{} }
func (omit) rowType() reflect.Type     { return nil }
func (o omit) compType() reflect.Type  { return o.t }
