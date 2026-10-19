package plugin

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
)

// Kept is a plugin's settings saved with the game, made by [Keep]: Persisted encodes them whole,
// so a setting saved at its zero value comes back so, not as the start it was given.
type Kept[T any] struct {
	of   *T
	blob []byte
}

// Keep has of, a struct of exported fields, saved and restored whole: a plugin's Serializable
// is Keep(&p.Res.Settings), its Restore calls Kept.Restore before it applies what came back.
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
