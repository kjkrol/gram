package world

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world/kind"
	"github.com/kjkrol/gram/plugins/world/kind/comp"
	"github.com/kjkrol/gram/render"
)

// Kinds is a Plugin's registered set of entity kinds — reached via Plugin.Kinds
// and handed to kind.Define, never built directly by the game.
type Kinds struct {
	entries map[string]registered
	next    render.SpriteID
	order   []string // live: kind.ID -> name, in Define order
	saved   []string // what a save brought in; empty on a fresh run

	families    map[reflect.Type]*tagFamily
	familyOrder []reflect.Type
	savedTags   map[string][]string // per family type name, what a save brought in
	heights     bool
}

// tagFamily is one family of tags: its names by bit, its component for saves, its remap.
type tagFamily struct {
	names []string
	load  goke.CompToken
	remap func(si *goke.SysInit, lut []uint8)
}

// registered is a kind as the world spawns it: its Position and Velocity picked
// out of the Spec, since those two live in Base rather than in columns of their own.
type registered struct {
	name     string
	typeID   kind.ID
	spriteID render.SpriteID
	row      reflect.Type
	position comp.Template[Position]
	velocity comp.Template[Velocity]
	comps    []comp.Comp
}

var (
	_ kind.Registry       = (*Kinds)(nil)
	_ goke.SetupProvider  = (*Kinds)(nil)
	_ goke.CompProvider   = (*Kinds)(nil)
	_ plugin.Serializable = (*Kinds)(nil)
)

func newKinds(heights bool) *Kinds {
	return &Kinds{entries: make(map[string]registered), families: make(map[reflect.Type]*tagFamily), heights: heights}
}

// Register takes spec on as name and assigns its ID and SpriteID by call order.
func (k *Kinds) Register(name string, row reflect.Type, spec kind.Spec) (kind.ID, render.SpriteID) {
	if len(k.order) == kind.MaxKinds {
		panic(fmt.Sprintf("world: cannot define kind %q: a world holds at most %d kinds", name, kind.MaxKinds))
	}
	if _, taken := k.entries[name]; taken {
		panic(fmt.Sprintf("world: kind %q is defined twice", name))
	}
	r := registered{name: name, typeID: kind.ID(len(k.order)), spriteID: k.NewSprite(), row: row}
	var positions, velocities int
	for _, c := range spec {
		if _, z := c.(comp.Template[Z]); z && !k.heights {
			panic(fmt.Sprintf("world: kind %q carries a Z in a flat world; set world.Config.Heights", name))
		}
		switch t := c.(type) {
		case comp.Template[Position]:
			r.position, positions = t, positions+1
		case comp.Template[Velocity]:
			r.velocity, velocities = t, velocities+1
		default:
			r.comps = append(r.comps, c)
		}
	}
	if positions != 1 || velocities != 1 {
		panic(fmt.Sprintf("world: kind %q names %d Position and %d Velocity components, want one of each", name, positions, velocities))
	}
	k.order = append(k.order, name)
	k.entries[name] = r
	return r.typeID, r.spriteID
}

// Reserve takes a kind.ID for entities spawned outside Populate — see Plugin.NewBodies.
func (k *Kinds) Reserve(name string) kind.ID {
	if len(k.order) == kind.MaxKinds {
		panic(fmt.Sprintf("world: cannot reserve kind %q: a world holds at most %d kinds", name, kind.MaxKinds))
	}
	if _, taken := k.entries[name]; taken {
		panic(fmt.Sprintf("world: kind %q is defined twice", name))
	}
	id := kind.ID(len(k.order))
	k.order = append(k.order, name)
	k.entries[name] = registered{name: name, typeID: id}
	return id
}

// DefineTag registers name in family F and returns its tag, assigned by call order within the
// family; a save records the names, so a build that defines them in another order still loads.
func (k *Kinds) DefineTag[F any](name string) plugin.Tag[F] {
	family := reflect.TypeFor[F]()
	f := k.families[family]
	if f == nil {
		f = &tagFamily{load: goke.LoadComp[plugin.Tags[F]](), remap: remapTags[F]}
		k.families[family] = f
		k.familyOrder = append(k.familyOrder, family)
	}
	for _, known := range f.names {
		if known == name {
			panic(fmt.Sprintf("world: tag %q is defined twice in family %v", name, family))
		}
	}
	if len(f.names) == plugin.MaxTagsPerFamily {
		panic(fmt.Sprintf("world: family %v holds at most %d tags", family, plugin.MaxTagsPerFamily))
	}
	f.names = append(f.names, name)
	return plugin.Tag[F](len(f.names) - 1)
}

// remapTags rewrites every loaded Tags[F] from the saved bit order to this build's.
func remapTags[F any](si *goke.SysInit, lut []uint8) {
	var tags goke.Comp[plugin.Tags[F]]
	query := si.NewQueryBuilder(&tags).Build()
	query.All()
	for query.Next() {
		cursor := query.Cursor()
		for i, old := range tags.Slice(cursor) {
			var fresh plugin.Tags[F]
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
func (k *Kinds) NewSprite() render.SpriteID {
	id := k.next
	k.next++
	return id
}

// SetupSystems returns no systems.
func (k *Kinds) SetupSystems() []goke.System { return nil }

// LoadComps lists every component type the defined kinds give their entities, each once.
func (k *Kinds) LoadComps() []goke.CompToken {
	var tokens []goke.CompToken
	listed := map[string]bool{}
	for _, name := range k.order {
		for _, c := range k.entries[name].comps {
			if token := c.LoadToken(); !listed[token.Name] {
				listed[token.Name] = true
				tokens = append(tokens, token)
			}
		}
	}
	for _, family := range k.familyOrder {
		if token := k.families[family].load; !listed[token.Name] {
			listed[token.Name] = true
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// Persisted returns the kind.ID to name mapping, and each tag family's bit to name mapping,
// for Persistence.Save and Load.
func (k *Kinds) Persisted() []any {
	k.saved = k.order
	k.savedTags = make(map[string][]string, len(k.families))
	for family, f := range k.families {
		k.savedTags[family.String()] = f.names
	}
	return []any{&k.saved, &k.savedTags}
}

// remapTags rewrites every loaded family's bits from the saved order to this build's.
func (k *Kinds) remapTags(si *goke.SysInit) {
	for _, family := range k.familyOrder {
		f := k.families[family]
		saved, ok := k.savedTags[family.String()]
		if !ok {
			continue
		}
		lut := make([]uint8, len(saved))
		moved := false
		for old, name := range saved {
			fresh := slices.Index(f.names, name)
			if fresh < 0 {
				panic(fmt.Sprintf("world: the save names tag %q in family %v, which this build no longer defines", name, family))
			}
			lut[old] = uint8(fresh)
			moved = moved || fresh != old
		}
		if moved {
			f.remap(si, lut)
		}
	}
}
