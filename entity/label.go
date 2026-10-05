package entity

import (
	"hash/fnv"
	"slices"
	"strings"
)

// Label is what an entity is called: its own name, borne by it alone, and the group it is in,
// with any number of others — each hashed, zero for none. A cell gets one from its cell.Entry, a
// unit from its kind.Entry; commands find their entities by it (Named, Group).
type Label struct{ Name, Group uint64 }

// LabelOf is the Label of an entity named name in the group group, either empty for none.
func LabelOf(name, group string) Label { return Label{Name: hashed(name), Group: hashed(group)} }

// hashed is a name as a Label keeps it; none for the empty name.
func hashed(name string) uint64 {
	if name == "" {
		return 0
	}
	h := fnv.New64a()
	h.Write([]byte(name))
	return h.Sum64()
}

// Whom says which entities a command is for, or who sets it off: those Named, those in a Group,
// or the World itself.
type Whom struct {
	world         bool
	names, groups []uint64
	said          []string
}

// World is the world's own entity: the state of the whole game.
var World = Whom{world: true, said: []string{"the world"}}

// Named are the entities bearing these names, one each.
func Named(names ...string) Whom {
	w := Whom{}
	for _, n := range names {
		w.names, w.said = append(w.names, hashed(n)), append(w.said, `"`+n+`"`)
	}
	return w
}

// Group are all the entities in these groups.
func Group(names ...string) Whom {
	w := Whom{}
	for _, n := range names {
		w.groups, w.said = append(w.groups, hashed(n)), append(w.said, `the group "`+n+`"`)
	}
	return w
}

// Target marks Whom as whom a command may be for.
func (Whom) Target() {}

// IsWorld reports whether it is the World.
func (w Whom) IsWorld() bool { return w.world }

// Nobody reports whether it names no one: the zero Whom.
func (w Whom) Nobody() bool { return !w.world && len(w.names) == 0 && len(w.groups) == 0 }

// Holds reports whether an entity labelled l is among those it names.
func (w Whom) Holds(l Label) bool {
	return l.Name != 0 && slices.Contains(w.names, l.Name) || l.Group != 0 && slices.Contains(w.groups, l.Group)
}

// Each calls fn with every name and group it names, as written, and whether a Label bears it.
func (w Whom) Each(fn func(said string, borne func(Label) bool)) {
	for i, n := range w.names {
		fn(w.said[i], func(l Label) bool { return l.Name == n })
	}
	for i, g := range w.groups {
		fn(w.said[i], func(l Label) bool { return l.Group == g })
	}
}

// String says whom, as written.
func (w Whom) String() string { return strings.Join(w.said, ", ") }
