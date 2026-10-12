package rule

import (
	"fmt"
	"reflect"
	"time"

	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/plugin"
)

// Branch is what is done when, and on whom: at the moment P a plugin's pass catches, or in a
// plan as the fact P comes, or for P any on nothing but its When. Given to a role it is a rule,
// given to a kind with Plan it is the plan or a branch of one; inside another's Do it asks more.
type Branch[P any] struct {
	Name string // required at the top of a rule and of a plan
	When any    // a predicate of P, a Cond, All of them; nil for always
	On   On     // required at the top; inside a Do, the outer one's unless said
	Do   Step
}

var _ Rule = Branch[struct{}]{}

func (Branch[P]) rule() {}

// String is the branch's name and its moment: "fall in" of unit.Standing.
func (b Branch[P]) String() string { return fmt.Sprintf("%q of %v", b.Name, reflect.TypeFor[P]()) }

func (b Branch[P]) narrowed(n narrowing) Rule { return b.build().narrowed(n) }

func (b Branch[P]) node() steps.Step {
	a := asked[P]{name: b.Name}
	a.take(b.When)
	return steps.NewBranch(b.Name, a.preds, a.guards, b.On.wrap(b.do(), false))
}

// topNode is the branch at the top of a plan, where it must say its name and whom it acts on.
func (b Branch[P]) topNode() steps.Step {
	b.check("plan")
	a := asked[P]{name: b.Name}
	a.take(b.When)
	return steps.NewBranch(b.Name, a.preds, a.guards, b.On.wrap(b.do(), true))
}

func (b Branch[P]) check(what string) {
	if b.Name == "" {
		panic(fmt.Sprintf("rule: a branch of %v at the top of a %s needs a Name",
			reflect.TypeFor[P](), what))
	}
	if b.On.kind == inherit {
		panic(fmt.Sprintf("rule: %q at the top of a %s needs an On", b.Name, what))
	}
}

func (b Branch[P]) do() steps.Step {
	if b.Do == nil {
		return steps.NewThen("")
	}
	return b.Do.node()
}

func (b Branch[P]) build() Rule {
	b.check("rule")
	a := asked[P]{name: b.Name, top: true}
	a.take(b.When)
	root := steps.NewBranch(b.Name, a.preds, a.guards, b.On.wrap(b.do(), true))
	roles := a.roles
	if _, met := any(*new(P)).(plugin.Met); met && len(roles) > 0 {
		a.sides.self = plugin.SideOf(roles[0].tag)
		roles = roles[1:]
	}
	r := construct[P](b.Name, a.sides, root)
	for _, role := range roles {
		r = within(role.tag, r, "")
	}
	return r
}

// Timeout is the branch failing, and stopped, once it has run for d; a plan's alone.
func (b Branch[P]) Timeout(d time.Duration) Framed { return Framed{b}.Timeout(d) }

// Every is the branch not beginning again within d of its last run; a plan's alone.
func (b Branch[P]) Every(d time.Duration) Framed { return Framed{b}.Every(d) }

// Framed is a Branch under a Timeout or an Every, which may take the other too.
type Framed struct{ inner Step }

func (f Framed) node() steps.Step { return f.inner.node() }

func (f Framed) topNode() steps.Step { return topNode(f.inner) }

// Timeout is the branch failing, and stopped, once it has run for d.
func (f Framed) Timeout(d time.Duration) Framed {
	return Framed{frame{f.inner, func(s steps.Step) steps.Step { return steps.NewTimeout(d, s) }}}
}

// Every is the branch not beginning again within d of its last run.
func (f Framed) Every(d time.Duration) Framed {
	return Framed{frame{f.inner, func(s steps.Step) steps.Step { return steps.NewCooldown(d, s) }}}
}

type frame struct {
	inner Step
	wrap  func(steps.Step) steps.Step
}

func (f frame) node() steps.Step    { return f.wrap(f.inner.node()) }
func (f frame) topNode() steps.Step { return f.wrap(topNode(f.inner)) }

// Hosted is the branch built into the rule its moment's plugin runs, for the hosts
// (plugin.Hosted).
func (b Branch[P]) Hosted() any { return b.build() }

// Build is r as its host takes it: a Branch built, any other rule as it is. It panics where
// the branch cannot be built.
func Build(r Rule) Rule {
	if b, ok := r.(interface{ build() Rule }); ok {
		return b.build()
	}
	return r
}
