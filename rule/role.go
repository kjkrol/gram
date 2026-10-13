package rule

import (
	"slices"

	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/rule/act"
	"github.com/kjkrol/gram/rule/cond"
)

// Part is a role an entity plays: the rules those playing it obey, given to a kind with Plays.
type Part struct {
	name  string
	tag   tag.Tag[cond.Roles]
	rules []act.Rule
}

// NewPart is the role named name with the tag t of cond.Roles, for the world's register of
// roles.
func NewPart(name string, t tag.Tag[cond.Roles]) *Part { return &Part{name: name, tag: t} }

// Obeys adds rules those playing the role obey, each narrowed to them.
func (r *Part) Obeys(rules ...act.Rule) *Part {
	for _, b := range rules {
		r.rules = append(r.rules, b.ForRole(r))
	}
	return r
}

// Tag is the role's tag of cond.Roles (cond.Role).
func (r *Part) Tag() tag.Tag[cond.Roles] { return r.tag }

// Rules are the rules the role's players obey.
func (r *Part) Rules() []act.Rule { return slices.Clone(r.rules) }

// String names the role.
func (r *Part) String() string { return "the role " + r.name }

// Hosted is the role itself: its host takes its rules one by one (plugin.Hosted).
func (r *Part) Hosted() any { return r }

// ForRole is the role's rules narrowed to the players of role too (act.Rule).
func (r *Part) ForRole(role cond.Role) act.Rule {
	q := *r
	q.rules = make([]act.Rule, len(r.rules))
	for i, b := range r.rules {
		q.rules[i] = b.ForRole(role)
	}
	return &q
}

// Plays is the component of an entity playing roles, for a kind's Spec: every role in one.
func Plays(roles ...*Part) Played {
	tags := make([]tag.Tag[cond.Roles], len(roles))
	for i, r := range roles {
		tags[i] = r.tag
	}
	return Played{Template: comp.Tagged(tags...), parts: slices.Clone(roles)}
}

// Played is what Plays makes: the roles' tags as a kind's component.
type Played struct {
	comp.Template[tag.Tags[cond.Roles]]
	parts []*Part
}

// Parts are the roles played.
func (p Played) Parts() []*Part { return slices.Clone(p.parts) }
