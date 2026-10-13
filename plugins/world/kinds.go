package world

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/plan"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	ikinds "github.com/kjkrol/gram/plugins/world/internal/kinds"
	"github.com/kjkrol/gram/render"
)

// Kinds is the world's register of entity kinds and tags, handed to kind.Define. Saves keep
// their names, so a build that defines them in another order still loads.
type Kinds struct {
	r      *ikinds.Registry
	played []act.Rule
	plans  *plan.Plans
	guard  func(name string)
}

var (
	_ kind.Registry       = (*Kinds)(nil)
	_ goke.SetupProvider  = (*Kinds)(nil)
	_ goke.CompProvider   = (*Kinds)(nil)
	_ plugin.Serializable = (*Kinds)(nil)
)

func newKinds(heights bool) *Kinds { return &Kinds{r: ikinds.New(heights)} }

// Register takes spec on as name and assigns its ID and SpriteID by call order.
func (k *Kinds) Register(name string, row reflect.Type, spec kind.Spec) (kind.ID, render.SpriteID) {
	if k.guard != nil {
		k.guard(name)
	}
	for _, c := range spec {
		switch c := c.(type) {
		case ceremony.Played:
			k.Play(c.Parts()...)
		case plan.Planned:
			if k.plans == nil {
				panic(fmt.Sprintf("world: the kind %q carries the plan %q, its world runs none",
					name, c.Name()))
			}
			k.plans.Register(c)
		}
	}
	return k.r.Register(name, row, spec)
}

// Lookup is the kind registered as name.
func (k *Kinds) Lookup(name string) (kind.ID, render.SpriteID, reflect.Type, bool) {
	return k.r.Lookup(name)
}

// Play notes roles played by what is no kind of the world's, a cell's kind or a plugin.
func (k *Kinds) Play(roles ...*ceremony.Part) {
	for _, part := range roles {
		if !slices.Contains(k.played, act.Rule(part)) {
			k.played = append(k.played, part)
		}
	}
}

// Played are the roles played so far, each once.
func (k *Kinds) Played() []act.Rule { return slices.Clone(k.played) }

// DefineTag registers name in family F and returns its tag, assigned by call order within the
// family.
func (k *Kinds) DefineTag[F any](name string) tag.Tag[F] { return ikinds.DefineTag[F](k.r, name) }

// NewSprite reserves an atlas slot that belongs to no kind.
func (k *Kinds) NewSprite() render.SpriteID { return k.r.NewSprite() }

// SetupSystems returns no systems.
func (k *Kinds) SetupSystems() []goke.System { return nil }

// LoadComps lists every component type the defined kinds give their entities, each once.
func (k *Kinds) LoadComps() []goke.CompToken { return k.r.LoadComps() }

// Persisted is the kinds' and the tags' names by order, for the saves.
func (k *Kinds) Persisted() []any { return k.r.Persisted() }
