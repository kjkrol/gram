package world

import (
	"reflect"
	"slices"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	ikinds "github.com/kjkrol/gram/plugins/world/internal/kinds"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
)

// Kinds is a Plugin's registered set of entity kinds and tags — reached via Plugin.Kinds and
// handed to kind.Define, never built directly by the game. Saves keep their names, so a build
// that defines them in another order still loads.
type Kinds struct {
	r      *ikinds.Registry
	played []*rule.Part // the roles its kinds play, each once
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
	for _, c := range spec {
		if p, ok := c.(rule.Played); ok {
			for _, part := range p.Parts() {
				if !slices.Contains(k.played, part) {
					k.played = append(k.played, part)
				}
			}
		}
	}
	return k.r.Register(name, row, spec)
}

// Played are the roles the kinds defined so far play, each once: what the engine hooks once a
// Stage's Init returns.
func (k *Kinds) Played() []*rule.Part { return slices.Clone(k.played) }

// DefineTag registers name in family F and returns its tag, assigned by call order within the
// family; a save records the names, so a build that defines them in another order still loads.
func (k *Kinds) DefineTag[F any](name string) tag.Tag[F] { return ikinds.DefineTag[F](k.r, name) }

// NewSprite reserves an atlas slot that belongs to no kind — an overlay's, say.
func (k *Kinds) NewSprite() render.SpriteID { return k.r.NewSprite() }

// SetupSystems returns no systems.
func (k *Kinds) SetupSystems() []goke.System { return nil }

// LoadComps lists every component type the defined kinds give their entities, each once.
func (k *Kinds) LoadComps() []goke.CompToken { return k.r.LoadComps() }

// Persisted returns the kinds' and the tags' names by order, for Persistence.Save and Load.
func (k *Kinds) Persisted() []any { return k.r.Persisted() }
