package act

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule/cond"
)

// Branch is when what is done, every clause saying of whom: at the moment P a plugin's pass
// catches, or for P any on nothing but its When. Given to a role it is a rule; inside another
// branch's Then it asks more. In a plan it is a loop that ends with its steps.
type Branch[P any] struct {
	Name string    // required at the top of a rule
	When cond.Cond // required: cond.Self.If(cond.Always) for every time
	Then Step      // required
}

// String is the branch's name and its moment: "fall in" of unit.Standing.
func (b Branch[P]) String() string { return fmt.Sprintf("%q of %v", b.Name, reflect.TypeFor[P]()) }

// Hosted is the branch built into the rule its moment's plugin runs, for the hosts
// (plugin.Hosted).
func (b Branch[P]) Hosted() any { return b.build() }

// ForRole is the rule narrowed to the players of role.
func (b Branch[P]) ForRole(role cond.Role) Rule { return b.build().ForRole(role) }

// Lasting is the branch as a plan takes it.
func (b Branch[P]) Lasting() Lasting { return b }

func (b Branch[P]) node(outer string, bd *build) steps.Step {
	name := b.Name
	if name == "" {
		name = outer
	}
	low := b.lower(name, steps.Inside, bd.moment)
	return bd.place(b, steps.NewBranch[P](name, low.Guard, b.Then.node(name, bd), nil), b.Then)
}

// nothing where When or Then is missing: lower says so by name
func (b Branch[P]) places() placed {
	if b.When == nil || b.Then == nil {
		return placed{}
	}
	low := b.When.(steps.Lowered).Lower(steps.Inside, b.Name)
	return placed{rings: low.Rings, count: low.Places}.with(b.Then.places())
}

// topNode is the branch at the top of a plan, where it must say its name.
func (b Branch[P]) topNode() steps.Step {
	b.check("plan")
	if p := b.places(); p.count > 0 {
		panic(fmt.Sprintf("act: %q: Around is a rule's: a plan has no places", b.Name))
	}
	return b.node(b.Name, &build{})
}

func (b Branch[P]) check(what string) {
	if b.Name == "" {
		panic(fmt.Sprintf("act: a branch of %v at the top of a %s needs a Name",
			reflect.TypeFor[P](), what))
	}
}

// in a rule a word must be of the rule's moment; a plan reads any fact's off the entity
func (b Branch[P]) lower(name string, as steps.LowerAs, of reflect.Type) steps.Lowering {
	if b.When == nil {
		panic(fmt.Sprintf("act: %q needs a When: cond.Self.If(cond.Always) for every time", name))
	}
	if b.Then == nil {
		panic(fmt.Sprintf("act: %q needs a Then", name))
	}
	low := b.When.(steps.Lowered).Lower(as, name)
	if of != nil {
		for _, w := range low.Words {
			if w != of {
				panic(fmt.Sprintf("act: %q: a word of %v in a rule of %v", name, w, of))
			}
		}
	}
	return low
}

func (b Branch[P]) build() Rule {
	b.check("rule")
	p := b.places()
	if p.mixed {
		panic(fmt.Sprintf("act: %q: one ring of places a rule", b.Name))
	}
	bd := &build{rings: p.rings, total: p.count, moment: reflect.TypeFor[P]()}
	low := b.lower(b.Name, steps.AtTop, bd.moment)
	root := bd.place(b, steps.NewBranch[P](b.Name, low.Guard, b.Then.node(b.Name, bd), nil),
		b.Then)
	var filter sides
	roles := low.SelfRoles
	_, met := any(*new(P)).(plugin.Met)
	if low.OtherRole >= 0 {
		if !met {
			panic(fmt.Sprintf("act: %q: the other's role needs a moment with others (Met)",
				b.Name))
		}
		filter.other = plugin.SideOf(tag.Tag[cond.Roles](low.OtherRole))
	}
	if met && len(roles) > 0 {
		filter.self = plugin.SideOf(tag.Tag[cond.Roles](roles[0]))
		roles = roles[1:]
	}
	r := construct[P](b.Name, filter, root)
	for _, role := range roles {
		r = within(tag.Tag[cond.Roles](role), r, "")
	}
	return r
}

// Build is r as its host takes it: a Branch built, any other rule as it is. It panics where
// the branch cannot be built.
func Build(r Rule) Rule {
	if b, ok := r.Hosted().(Rule); ok {
		return b
	}
	return r
}
