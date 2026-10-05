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
	played []rule.Rule       // the roles played, each once
	guard  func(name string) // panics for a kind defined out of its section; nil for none
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
		if p, ok := c.(rule.Played); ok {
			k.Play(p.Parts()...)
		}
	}
	return k.r.Register(name, row, spec)
}

// Play notes roles somebody plays who is no kind of the world's — a cell's kind, a plugin: for
// the plugins, as a kind's own are noted when it is defined.
func (k *Kinds) Play(roles ...*rule.Part) {
	for _, part := range roles {
		if !slices.Contains(k.played, rule.Rule(part)) {
			k.played = append(k.played, part)
		}
	}
}

// Played are the roles played so far, each once: what the engine hands the hosts of their rules'
// moments once a Stage's Init returns.
func (k *Kinds) Played() []rule.Rule { return slices.Clone(k.played) }

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
