package plugin

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"reflect"
)

// Kept is a plugin's settings saved with the game, made by [Keep]: Persisted encodes them whole,
// so a setting saved at its zero value comes back so, not as the start it was given.
type Kept[T any] struct {
	of   *T
	blob []byte
}

// Keep has of, a struct of exported fields, saved and restored whole: a plugin's Serializable
// is Keep(&p.Res), or Keep(&p.Res.Settings) where the rest of its Res is not saved; its Restore
// calls Kept.Restore before it applies what came back.
func Keep[T any](of *T) *Kept[T] { return &Kept[T]{of: of} }

var _ Serializable = (*Kept[struct{}])(nil)
var _ Restorer = (*Kept[struct{}])(nil)

// Persisted is the settings encoded, one value for Persistence.
func (k *Kept[T]) Persisted() []any {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(k.of); err != nil {
		panic(fmt.Sprintf("plugin: keep %T: %v", *k.of, err))
	}
	k.blob = buf.Bytes()
	return []any{&k.blob}
}

// Restore replaces the settings with what was decoded; a save without them leaves them as they
// are.
func (k *Kept[T]) Restore() {
	if len(k.blob) == 0 {
		return
	}
	var v T
	if err := gob.NewDecoder(bytes.NewReader(k.blob)).Decode(&v); err != nil {
		log.Printf("plugin: kept %T not restored: %v", v, err)
		return
	}
	*k.of = v
}

// KeepOf is Keep of a value whose type the caller does not name: of is a pointer to a struct of
// exported fields, saved and restored whole; anything else panics.
func KeepOf(of any) Serializable {
	v := reflect.ValueOf(of)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		panic(fmt.Sprintf("plugin: KeepOf wants a pointer to a struct of exported fields, not %T",
			of))
	}
	return &keptOf{of: v}
}

type keptOf struct {
	of   reflect.Value
	blob []byte
}

var _ Serializable = (*keptOf)(nil)
var _ Restorer = (*keptOf)(nil)

// Name tells the kept structs apart in a save: the struct's type, not the keeper's
func (k *keptOf) Name() string { return k.of.Elem().Type().String() }

func (k *keptOf) Persisted() []any {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(k.of.Interface()); err != nil {
		panic(fmt.Sprintf("plugin: keep %s: %v", k.of.Elem().Type(), err))
	}
	k.blob = buf.Bytes()
	return []any{&k.blob}
}

func (k *keptOf) Restore() {
	if len(k.blob) == 0 {
		return
	}
	v := reflect.New(k.of.Elem().Type())
	if err := gob.NewDecoder(bytes.NewReader(k.blob)).Decode(v.Interface()); err != nil {
		log.Printf("plugin: kept %s not restored: %v", k.of.Elem().Type(), err)
		return
	}
	k.of.Elem().Set(v.Elem())
}
