package world

import (
	"fmt"

	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/act"
	"github.com/kjkrol/gram/rule/cond"
)

// Roles is a Stage's register of roles, by name. A Stage names 64 roles at most.
type Roles struct {
	w      *Plugin
	byName map[string]*rule.Part
}

// Define registers the role called name and the rules those playing it obey. A name defined
// twice panics; a rule no host takes fails Init whether anybody plays the role or not.
func (r *Roles) Define(name string, rules ...act.Rule) {
	r.w.must(fmt.Sprintf("role %q defined", name), section.Rules)
	if name == "" {
		panic("world: a role needs a name")
	}
	if _, ok := r.byName[name]; ok {
		panic(fmt.Sprintf("world: the role %q is defined already", name))
	}
	if r.byName == nil {
		r.byName = map[string]*rule.Part{}
	}
	part := rule.NewPart(name, r.w.kinds.DefineTag[cond.Roles](name)).Obeys(rules...)
	r.byName[name] = part
	r.w.kinds.Play(part)
}

// Lookup is the role defined as name; false for none.
func (r *Roles) Lookup(name string) (*rule.Part, bool) {
	part, ok := r.byName[name]
	return part, ok
}

// Named is the role defined as name; an unknown name panics.
func (r *Roles) Named(name string) *rule.Part {
	part, ok := r.byName[name]
	if !ok {
		panic(fmt.Sprintf("world: no role is defined as %q", name))
	}
	return part
}
