// Package kinds is the world's register of kinds and tags: what kind.Define and DefineTag
// register, the atlas slots, the names a save keeps, and the remapping of a load to this build's
// order. The world's Kinds hands a game its part of it.
package kinds

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/render"
)

// Registry is the world's kinds and tag families.
type Registry struct {
	entries map[string]Kind
	next    render.SpriteID
	order   []string // live: kind.ID -> name, in Define order
	saved   []string // what a save brought in; empty on a fresh run

	families    map[reflect.Type]*family
	familyOrder []reflect.Type
	savedTags   map[string][]string // per family type name, what a save brought in
	heights     bool
}

// Kind is a kind as the world spawns it: its Position and Velocity picked out of the Spec, since
// those two live in Base rather than in columns of their own.
type Kind struct {
	Name     string
	TypeID   kind.ID
	SpriteID render.SpriteID
	Row      reflect.Type
	Position comp.Template[entity.Position]
	Velocity comp.Template[entity.Velocity]
	Comps    []comp.Comp
}

// family is one family of tags: its names by bit, its component for saves, its remap.
type family struct {
	names []string
	load  goke.CompToken
	remap func(si *goke.SysInit, lut []uint8)
}

// New is an empty register; heights says whether the world takes a Z.
func New(heights bool) *Registry {
	return &Registry{entries: make(map[string]Kind), families: make(map[reflect.Type]*family), heights: heights}
}

// Register takes spec on as name and assigns its ID and SpriteID by call order.
func (k *Registry) Register(name string, row reflect.Type, spec kind.Spec) (kind.ID, render.SpriteID) {
	if len(k.order) == kind.MaxKinds {
		panic(fmt.Sprintf("world: cannot define kind %q: a world holds at most %d kinds", name, kind.MaxKinds))
	}
	if _, taken := k.entries[name]; taken {
		panic(fmt.Sprintf("world: kind %q is defined twice", name))
	}
	r := Kind{Name: name, TypeID: kind.ID(len(k.order)), SpriteID: k.NewSprite(), Row: row}
	var positions, velocities int
	seen := make(map[reflect.Type]bool, len(spec))
	for _, c := range spec {
		if t := comp.TypeOf(c); seen[t] {
			panic(fmt.Sprintf("world: kind %q carries %v twice; give each component once", name, t))
		} else {
			seen[t] = true
		}
		if _, z := c.(comp.Template[entity.Z]); z && !k.heights {
			panic(fmt.Sprintf("world: kind %q carries a Z in a flat world; set world.Config.Heights", name))
		}
		switch t := c.(type) {
		case comp.Template[entity.Position]:
			r.Position, positions = t, positions+1
		case comp.Template[entity.Velocity]:
			r.Velocity, velocities = t, velocities+1
		default:
			r.Comps = append(r.Comps, c)
		}
	}
	if positions != 1 || velocities != 1 {
		panic(fmt.Sprintf("world: kind %q names %d Position and %d Velocity components, want one of each", name, positions, velocities))
	}
	k.order = append(k.order, name)
	k.entries[name] = r
	return r.TypeID, r.SpriteID
}

// Lookup is the kind registered as name — see kind.Registry.
func (k *Registry) Lookup(name string) (kind.ID, render.SpriteID, reflect.Type, bool) {
	r, ok := k.entries[name]
	return r.TypeID, r.SpriteID, r.Row, ok
}

// Kind is the kind registered as name.
func (k *Registry) Kind(name string) (Kind, bool) {
	r, ok := k.entries[name]
	return r, ok
}

// Each calls fn with every kind, in the order defined: their TypeIDs count up from 0.
func (k *Registry) Each(fn func(Kind)) {
	for _, name := range k.order {
		fn(k.entries[name])
	}
}

// DefineTag registers name in family F and returns its tag, assigned by call order within the
// family.
func DefineTag[F any](k *Registry, name string) tag.Tag[F] {
	t := reflect.TypeFor[F]()
	f := k.families[t]
	if f == nil {
		f = &family{load: goke.LoadComp[tag.Tags[F]](), remap: remapTags[F]}
		k.families[t] = f
		k.familyOrder = append(k.familyOrder, t)
	}
	for _, known := range f.names {
		if known == name {
			panic(fmt.Sprintf("world: tag %q is defined twice in family %v", name, t))
		}
	}
	if len(f.names) == tag.MaxTagsPerFamily {
		panic(fmt.Sprintf("world: family %v holds at most %d tags", t, tag.MaxTagsPerFamily))
	}
	f.names = append(f.names, name)
	return tag.Tag[F](len(f.names) - 1)
}

// remapTags rewrites every loaded Tags[F] from the saved bit order to this build's.
func remapTags[F any](si *goke.SysInit, lut []uint8) {
	var tags goke.Comp[tag.Tags[F]]
	query := si.NewQueryBuilder(&tags).Build()
	query.All()
	for query.Next() {
		cursor := query.Cursor()
		for i, old := range tags.Slice(cursor) {
			var fresh tag.Tags[F]
			for bit := range lut {
				if old&(1<<bit) != 0 {
					fresh |= 1 << lut[bit]
				}
			}
			tags.Slice(cursor)[i] = fresh
		}
	}
}

// NewSprite reserves an atlas slot that belongs to no kind — an overlay's, say.
func (k *Registry) NewSprite() render.SpriteID {
	id := k.next
	k.next++
	return id
}

// LoadComps lists every component type the defined kinds give their entities, each once.
func (k *Registry) LoadComps() []goke.CompToken {
	var tokens []goke.CompToken
	listed := map[string]bool{}
	for _, name := range k.order {
		for _, c := range k.entries[name].Comps {
			if token := c.LoadToken(); !listed[token.Name] {
				listed[token.Name] = true
				tokens = append(tokens, token)
			}
		}
	}
	for _, f := range k.familyOrder {
		if token := k.families[f].load; !listed[token.Name] {
			listed[token.Name] = true
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// Persisted returns the kind.ID to name mapping, and each tag family's bit to name mapping, for
// a save to keep and a load to fill.
func (k *Registry) Persisted() []any {
	k.saved = slices.Clone(k.order) // a load decodes into it: never this build's own order
	k.savedTags = make(map[string][]string, len(k.families))
	for t, f := range k.families {
		k.savedTags[t.String()] = f.names
	}
	return []any{&k.saved, &k.savedTags}
}

// RemapTypes rewrites every loaded Base.TypeID from the saved kind order to this build's.
func (k *Registry) RemapTypes(si *goke.SysInit) {
	lut := make([]kind.ID, len(k.saved))
	moved := false
	for old, name := range k.saved {
		r, ok := k.entries[name]
		if !ok {
			panic(fmt.Sprintf("world: the save names kind %q, which this build no longer defines", name))
		}
		lut[old] = r.TypeID
		moved = moved || r.TypeID != kind.ID(old)
	}
	if !moved {
		return
	}

	var base goke.Comp[entity.Base]
	query := si.NewQueryBuilder(&base).Build()
	query.All()
	for query.Next() {
		cursor := query.Cursor()
		bases := base.Slice(cursor)
		for i := range cursor.IDs {
			if int(bases[i].TypeID) >= len(lut) {
				panic(fmt.Sprintf("world: loaded entity carries TypeID %d, beyond the %d the save named", bases[i].TypeID, len(lut)))
			}
			bases[i].TypeID = lut[bases[i].TypeID]
		}
	}
}

// RemapTags rewrites every loaded family's bits from the saved order to this build's.
func (k *Registry) RemapTags(si *goke.SysInit) {
	for _, t := range k.familyOrder {
		f := k.families[t]
		saved, ok := k.savedTags[t.String()]
		if !ok {
			continue
		}
		lut := make([]uint8, len(saved))
		moved := false
		for old, name := range saved {
			fresh := slices.Index(f.names, name)
			if fresh < 0 {
				panic(fmt.Sprintf("world: the save names tag %q in family %v, which this build no longer defines", name, t))
			}
			lut[old] = uint8(fresh)
			moved = moved || fresh != old
		}
		if moved {
			f.remap(si, lut)
		}
	}
}
