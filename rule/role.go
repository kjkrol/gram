package rule

import (
	"slices"

	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
)

// Roles is the family of roles: one tag.Tags[Roles] on an entity holds every role it plays.
type Roles struct{}

// Part is a role an entity plays: the rules those playing it obey, given to a kind with Plays.
type Part struct {
	name  string
	tag   tag.Tag[Roles]
	rules []Rule
}

// NewPart is the role named name with the tag t of Roles, for the world's register of roles.
func NewPart(name string, t tag.Tag[Roles]) *Part { return &Part{name: name, tag: t} }

// Obeys adds rules those playing the role obey, each narrowed to them.
func (r *Part) Obeys(rules ...Rule) *Part {
	for _, b := range rules {
		r.rules = append(r.rules, within(r.tag, Build(b), "for the role "+r.name))
	}
	return r
}

// Tag is the role's tag of Roles.
func (r *Part) Tag() tag.Tag[Roles] { return r.tag }

// Rules are the rules the role's players obey.
func (r *Part) Rules() []Rule { return slices.Clone(r.rules) }

// String names the role.
func (r *Part) String() string { return "the role " + r.name }

func (*Part) rule() {}

func (r *Part) narrowed(n narrowing) Rule {
	q := *r
	q.rules = make([]Rule, len(r.rules))
	for i, b := range r.rules {
		q.rules[i] = b.narrowed(n)
	}
	return &q
}

// Plays is the component of an entity playing roles, for a kind's Spec: every role in one.
func Plays(roles ...*Part) Played {
	tags := make([]tag.Tag[Roles], len(roles))
	for i, r := range roles {
		tags[i] = r.tag
	}
	return Played{Template: comp.Tagged(tags...), parts: slices.Clone(roles)}
}

// Played is what Plays makes: the roles' tags as a kind's component.
type Played struct {
	comp.Template[tag.Tags[Roles]]
	parts []*Part
}

// Parts are the roles played.
func (p Played) Parts() []*Part { return slices.Clone(p.parts) }
