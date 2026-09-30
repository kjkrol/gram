package kind

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
)

// Roster is what a world's plugins ask of the kinds a game defines, gathered as the plugins are
// made; a game builds a kind's Spec through a Role and hears at once what it left out.
type Roster struct{ Unit Role }

// NewRoster is an empty roster; a world makes one and its plugins fill it.
func NewRoster() *Roster { return &Roster{Unit: Role{name: "unit"}} }

// Role is one shape of entity: what plugins require of it (the game supplies) and what they
// bring themselves (a default the game may override or drop).
type Role struct {
	name     string
	needs    []need
	defaults []comp.Comp
}

type need struct {
	comp    reflect.Type
	by, why string
}

// Require says plugin by needs every entity of the role to carry a T, and why.
func Require[T any](r *Role, by, why string) {
	r.needs = append(r.needs, need{comp: reflect.TypeFor[T](), by: by, why: why})
}

// Default is a component every entity of the role carries unless the game gives its own of
// that type or leaves it out with comp.Without.
func (r *Role) Default(c comp.Comp) { r.defaults = append(r.defaults, c) }

// Spec is the role's defaults, less those own overrides or drops, plus own; it panics naming
// every requirement own leaves unmet.
func (r *Role) Spec(own ...comp.Comp) Spec {
	given := map[reflect.Type]bool{}
	for _, c := range own {
		given[comp.TypeOf(c)] = true
	}
	spec := make(Spec, 0, len(r.defaults)+len(own))
	for _, c := range r.defaults {
		if !given[comp.TypeOf(c)] {
			spec = append(spec, c)
		}
	}
	for _, c := range own {
		if _, left := comp.Omitted(c); !left {
			spec = append(spec, c)
		}
	}
	var missing []string
	for _, n := range r.needs {
		if !given[n.comp] {
			missing = append(missing, fmt.Sprintf("%s requires %v (%s)", n.by, n.comp, n.why))
		}
	}
	if len(missing) > 0 {
		panic(fmt.Sprintf("kind: %s: %s", r.name, strings.Join(missing, "; ")))
	}
	return spec
}
