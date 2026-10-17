package kind

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/kjkrol/ceremony/comp"
)

// Template is a sort of kind a plugin brings in by name — what the world spawns, what moves, a
// board's cell — the asks of its parent coming with its own.
type Template struct {
	name   string
	parent *Template
}

// NewTemplate is a template named name under parent, nil for none: a handle, its asks a world's
// Roster's.
func NewTemplate(name string, parent *Template) *Template {
	return &Template{name: name, parent: parent}
}

// Roster is what the plugins of a world ask of the kinds a game defines, by template: written by
// every plugin's KindTemplates as it is first read, never after.
type Roster struct {
	asks    map[*Template]*asks
	fill    func(r *Roster)
	writing bool
	filled  bool
}

// what the plugins ask of a template's kinds
type asks struct {
	needs    []need
	defaults []comp.Comp
}

type need struct {
	comp    reflect.Type
	by, why string
}

// NewRoster is a roster fill writes as it is first read, the plugins' asks in the order made.
func NewRoster(fill func(r *Roster)) *Roster {
	return &Roster{asks: map[*Template]*asks{}, fill: fill}
}

// Filled reports whether the roster has been written, its plugins asked.
func (r *Roster) Filled() bool { return r.filled }

// Require says plugin by needs every entity of a kind of template t to carry a T, and why.
func Require[T any](r *Roster, t *Template, by, why string) {
	a := r.write(t, fmt.Sprintf("a %v", reflect.TypeFor[T]()))
	a.needs = append(a.needs, need{comp: reflect.TypeFor[T](), by: by, why: why})
}

// Default is a component every entity of a kind of template t carries unless the game gives its
// own of that type or leaves it out with comp.Without.
func (r *Roster) Default(t *Template, c comp.Comp) {
	a := r.write(t, fmt.Sprintf("a default %v", c.Type()))
	a.defaults = append(a.defaults, c)
}

// Spec is template t's defaults and its parents', less those own overrides or drops, plus own; it
// panics naming every requirement own leaves unmet. A template's default takes the place of its
// parent's of the same type.
func (r *Roster) Spec(t *Template, own ...comp.Comp) Spec {
	r.read()
	given := map[reflect.Type]bool{}
	for _, c := range own {
		given[c.Type()] = true
	}
	var spec Spec
	var missing []string
	for at := t; at != nil; at = at.parent {
		a := r.of(at)
		for _, c := range a.defaults {
			if !given[c.Type()] {
				given[c.Type()] = true
				spec = append(spec, c)
			}
		}
		for _, n := range a.needs {
			if !given[n.comp] {
				missing = append(missing, fmt.Sprintf("%s requires %v (%s)", n.by, n.comp, n.why))
			}
		}
	}
	for _, c := range own {
		if _, left := comp.Omitted(c); !left {
			spec = append(spec, c)
		}
	}
	if len(missing) > 0 {
		panic(fmt.Sprintf("kind: %s: %s", t.name, strings.Join(missing, "; ")))
	}
	return spec
}

func (r *Roster) of(t *Template) *asks {
	a, ok := r.asks[t]
	if !ok {
		a = &asks{}
		r.asks[t] = a
	}
	return a
}

func (r *Roster) read() {
	if r.writing {
		panic("kind: the roster read while the plugins' KindTemplates write it")
	}
	if r.filled {
		return
	}
	r.filled, r.writing = true, true
	if r.fill != nil {
		r.fill(r)
	}
	r.writing = false
}

func (r *Roster) write(t *Template, what string) *asks {
	if !r.writing {
		panic(fmt.Sprintf("kind: %s asked of the %s template outside a plugin's KindTemplates",
			what, t.name))
	}
	return r.of(t)
}
