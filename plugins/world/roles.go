package world

import (
	"fmt"

	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/rule"
)

// Roles are the roles of a Stage, by name: Define says a role and the rules those playing it
// obey, Named hands it to whoever gives it to a kind (rule.Plays), to a kind of cell
// (board.Plugin.Plays) or to a plugin (Self.Plays), or names it in a rule (rule.Other,
// rule.Playing). Reached through Plugin.Roles; each Stage's world has its own, so one name may
// mean one thing in one Stage and another in the next. A Stage names 64 roles at most.
type Roles struct {
	w      *Plugin
	byName map[string]*rule.Part
}

// Define says the role called name and the rules those playing it obey, none for a role that
// only says who somebody is. Call it where the Stage defines its rules; a name defined twice
// panics.
func (r *Roles) Define(name string, rules ...rule.Rule) {
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
	r.byName[name] = rule.NewPart(name, r.w.kinds.DefineTag[rule.Roles](name)).Obeys(rules...)
}

// Named is the role defined as name; an unknown name panics.
func (r *Roles) Named(name string) *rule.Part {
	part, ok := r.byName[name]
	if !ok {
		panic(fmt.Sprintf("world: no role is defined as %q", name))
	}
	return part
}
