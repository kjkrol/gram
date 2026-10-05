package kind

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/render"
)

// Spec is the definition of one kind: the components every entity of it carries — see package comp.
type Spec []comp.Comp

// ID identifies an entity's kind at runtime, carried on every entity a world spawns —
// the one trace of its kind that outlives spawning. Define assigns one per kind, in call order.
type ID uint8

// MaxKinds is how many kinds one Registry can hold, bounded by ID.
const MaxKinds = 1 << 8

// Registry is where kinds are kept — a world's, reached through world.Plugin.Kinds.
type Registry interface {
	Register(name string, row reflect.Type, spec Spec) (ID, render.SpriteID)
	// Lookup is the kind registered as name: its ID, its sprite and the type of its rows.
	Lookup(name string) (id ID, sprite render.SpriteID, row reflect.Type, ok bool)
}

// Of is one defined kind, whose entities are each described by a row of type P.
type Of[P any] struct {
	name   string
	id     ID
	sprite render.SpriteID
}

// Define registers spec under name as a kind whose rows are P; a Load of another row type panics.
// It hands nothing back: Named is the kind, for whoever spawns its entities.
func Define[P any](reg Registry, name string, spec Spec) {
	row := reflect.TypeFor[P]()
	for _, c := range spec {
		if read := comp.RowOf(c); read != nil && read != row {
			panic(fmt.Sprintf("kind: %q: a Load reads %v, but its rows are %v", name, read, row))
		}
	}
	reg.Register(name, row, spec)
}

// Named is the kind defined as name, whose rows are P; an unknown name, or a kind of other rows,
// panics.
func Named[P any](reg Registry, name string) Of[P] {
	id, sprite, row, ok := reg.Lookup(name)
	if !ok {
		panic(fmt.Sprintf("kind: no kind is defined as %q", name))
	}
	if want := reflect.TypeFor[P](); row != want {
		panic(fmt.Sprintf("kind: %q is asked for with rows of %v, but its rows are %v", name, want, row))
	}
	return Of[P]{name: name, id: id, sprite: sprite}
}

// Entry is one entity of this kind, described by row — hand it to world.Plugin.Seed.
func (k Of[P]) Entry(row P) Entry { return Entry{kind: k.name, row: row} }

// ID is what every entity of this kind carries to say so.
func (k Of[P]) ID() ID { return k.id }

// Name is what the kind was defined as.
func (k Of[P]) Name() string { return k.name }

// SpriteID is the atlas slot this kind's entities are drawn from.
func (k Of[P]) SpriteID() render.SpriteID { return k.sprite }

// Entry is one entity to spawn: its kind and its row — built only by Of.Entry.
type Entry struct {
	kind        string
	row         any
	name, group string
	told        []any
}

// Named is the entry of an entity bearing name, its alone: what entity.Named finds it by.
func (e Entry) Named(name string) Entry { e.name = name; return e }

// InGroup is the entry of an entity in group, with any number of others: what entity.Group finds
// it by.
func (e Entry) InGroup(group string) Entry { e.group = group; return e }

// Told is the entry of an entity given cmds as it is made — the commands it gives itself then:
// whose it is (players.Give), whether it may be selected (selection.Allow).
func (e Entry) Told(cmds ...any) Entry {
	e.told = append(append([]any(nil), e.told...), cmds...)
	return e
}

// Commands are the commands the entity gives itself as it is made.
func (e Entry) Commands() []any { return e.told }

// Name is the name the entity bears, empty for none; Group the group it is in.
func (e Entry) Name() string { return e.name }

// Group is the group the entity is in, empty for none.
func (e Entry) Group() string { return e.group }

// Kind names the kind this entity is of.
func (e Entry) Kind() string { return e.kind }

// Row is what describes this entity to its kind's Loads.
func (e Entry) Row() any { return e.row }
