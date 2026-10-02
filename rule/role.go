package rule

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/rule/effect"
)

// Roles is the family of roles: one tag.Tags[Roles] on an entity holds every role it plays.
type Roles struct{}

// Role is a part an entity plays: a tag of Roles, the rules those playing it obey (Obeys) and what
// they can do when their player asks (Can). A role means a behaviour — mortal, hasty, a trapdoor —
// not a group: a game has 64 at most. Define one by name with world.Plugin.Role.
type Role struct {
	name      string
	tag       tag.Tag[Roles]
	rules     []Rule
	abilities []Ability
}

// Ability is what a role can do when its player asks: put Effect on the player's selected units
// that play the role, bound to Trigger and listed under Label.
type Ability struct {
	Effect  effect.Effect
	Trigger control.Trigger
	Label   string
	Role    *Role
}

// NewRole is the role named name, its tag t: what world.Plugin.Role makes.
func NewRole(name string, t tag.Tag[Roles]) *Role { return &Role{name: name, tag: t} }

// Name is the role's name.
func (r *Role) Name() string { return r.name }

// Tag is the role's tag of Roles.
func (r *Role) Tag() tag.Tag[Roles] { return r.tag }

// Obeys adds rules those playing the role obey: each Within the role's tag, hooked with Rules.
func (r *Role) Obeys(rules ...Rule) *Role {
	for _, b := range rules {
		r.rules = append(r.rules, Within(r.tag, b))
	}
	return r
}

// Can adds an ability: the player's trigger puts e on its selected units playing the role, label
// saying so in the list of keys. A plugin carrying abilities out binds them (selection.Plugin.Abilities).
func (r *Role) Can(e effect.Effect, trigger control.Trigger, label string) *Role {
	r.abilities = append(r.abilities, Ability{Effect: e, Trigger: trigger, Label: label, Role: r})
	return r
}

// Rules are the rules the role's players obey, for game.Initializer.Hook.
func (r *Role) Rules() []Rule { return r.rules }

// Abilities are what the role can do.
func (r *Role) Abilities() []Ability { return r.abilities }

// RulesOf are the rules of every role, for game.Initializer.Hook.
func RulesOf(roles ...*Role) []Rule {
	var rules []Rule
	for _, r := range roles {
		rules = append(rules, r.rules...)
	}
	return rules
}

// Plays is the component of an entity playing roles, for a kind's Spec: every role it plays in
// one, so a kind names Plays once.
func Plays(roles ...*Role) comp.Template[tag.Tags[Roles]] {
	tags := make([]tag.Tag[Roles], len(roles))
	for i, r := range roles {
		tags[i] = r.tag
	}
	return comp.Tagged(tags...)
}
