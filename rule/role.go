package rule

import (
	"fmt"
	"slices"
	"sync"

	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
)

// Roles is the family of roles: one tag.Tags[Roles] on an entity holds every role it plays.
type Roles struct{}

// Part is a role an entity plays, as Role makes it: the rules those playing it obey (Obeys). Give
// it to a kind with Plays, to a kind of cell with board.Plugin.Plays, to a plugin with its own
// Plays: the engine hands the rules of a role somebody plays to the plugins catching their moments.
type Part struct {
	name  string
	tag   tag.Tag[Roles]
	rules []Rule
}

// Role is the role named name, ready for its rules. A role means a behaviour —
// mortal, hasty, a trapdoor — not a group: a program names 64 at most. Each call is a new Part;
// one name is one tag, which a world saves by the name.
func Role(name string) *Part { return &Part{name: name, tag: roleTag(name)} }

// names are the roles' names in the order of their tags, for every Part a program makes.
var names = struct {
	sync.Mutex
	order []string
	tags  map[string]tag.Tag[Roles]
}{tags: map[string]tag.Tag[Roles]{}}

// roleTag is name's tag, given at its first Role.
func roleTag(name string) tag.Tag[Roles] {
	names.Lock()
	defer names.Unlock()
	if t, ok := names.tags[name]; ok {
		return t
	}
	if len(names.order) == tag.MaxTagsPerFamily {
		panic(fmt.Sprintf("rule: cannot define the role %q: a program names at most %d roles", name, tag.MaxTagsPerFamily))
	}
	t := tag.Tag[Roles](len(names.order))
	names.order = append(names.order, name)
	names.tags[name] = t
	return t
}

// RoleNames are the names of the roles defined so far, in the order of their tags: what a world
// names in its kinds, so a save carries the roles by name.
func RoleNames() []string {
	names.Lock()
	defer names.Unlock()
	return append([]string(nil), names.order...)
}

// Obeys adds rules those playing the role obey, each for them alone.
func (r *Part) Obeys(rules ...Rule) *Part {
	for _, b := range rules {
		r.rules = append(r.rules, within(r.tag, b, "for the role "+r.name))
	}
	return r
}

// Tag is the role's tag of Roles: for the plugins giving it and reading it.
func (r *Part) Tag() tag.Tag[Roles] { return r.tag }

// Rules are the rules the role's players obey: what the engine hands the plugins.
func (r *Part) Rules() []Rule { return slices.Clone(r.rules) }

// String names the role.
func (r *Part) String() string { return "the role " + r.name }

func (*Part) rule() {}

// narrowed is the role whose rules are narrowed by n besides.
func (r *Part) narrowed(n narrowing) Rule {
	q := *r
	q.rules = make([]Rule, len(r.rules))
	for i, b := range r.rules {
		q.rules[i] = b.narrowed(n)
	}
	return &q
}

// Plays is the component of an entity playing roles, for a kind's Spec: every role it plays in
// one, so a kind names Plays once. The rules of a role some kind plays reach the plugins catching
// their moments once the Stage's Init returns.
func Plays(roles ...*Part) Played {
	tags := make([]tag.Tag[Roles], len(roles))
	for i, r := range roles {
		tags[i] = r.tag
	}
	return Played{Template: comp.Tagged(tags...), parts: slices.Clone(roles)}
}

// Played is what Plays makes: the roles' tags as a kind's component, the roles kept beside them.
type Played struct {
	comp.Template[tag.Tags[Roles]]
	parts []*Part
}

// Parts are the roles played: for the world, which tells the engine whose rules to hand over.
func (p Played) Parts() []*Part { return slices.Clone(p.parts) }
