package effects

import (
	"reflect"

	"github.com/kjkrol/uid"
)

// Saved is one altered component's original value, kept until the last Alter of it ends.
type Saved struct {
	Type  string
	Value any
}

// originals is every entity's saved originals — the plugin's own state, saved with the game.
type originals struct {
	byEntity map[uid.UID64][]Saved
}

func newOriginals() *originals { return &originals{byEntity: map[uid.UID64][]Saved{}} }

func (o *originals) get(id uid.UID64, comp reflect.Type) (any, bool) {
	for _, s := range o.byEntity[id] {
		if s.Type == comp.String() {
			return s.Value, true
		}
	}
	return nil, false
}

func (o *originals) put(id uid.UID64, comp reflect.Type, v any) {
	o.byEntity[id] = append(o.byEntity[id], Saved{Type: comp.String(), Value: v})
}

func (o *originals) drop(id uid.UID64, comp reflect.Type) {
	saved := o.byEntity[id]
	for i, s := range saved {
		if s.Type == comp.String() {
			o.byEntity[id] = append(saved[:i], saved[i+1:]...)
			break
		}
	}
	if len(o.byEntity[id]) == 0 {
		delete(o.byEntity, id)
	}
}

func (o *originals) forget(id uid.UID64) { delete(o.byEntity, id) }
