package rule

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Cond is a condition in a Branch's When besides a predicate of its moment or fact: of the
// entity's state, the world's or the other of a pair.
type Cond interface{ cond() }

// All holds where every one of conds, predicates and Conds alike, holds.
func All(conds ...any) Cond { return all{conds} }

// Not is the predicate holding where holds does not: When: rule.Not(unit.Standing.Fallen).
func Not[P any](holds func(P) bool) func(P) bool { return func(p P) bool { return !holds(p) } }

// Under holds while the entity is under the effect.
func Under(e effect.Effect) Cond { return guard{steps.GuardUnder(e)} }

// Free holds while the entity is under none of the effects.
func Free(effects ...effect.Effect) Cond {
	conds := make([]any, len(effects))
	for i, e := range effects {
		conds[i] = guard{steps.GuardFree(e)}
	}
	return all{conds}
}

// During holds while the world's own entity is under the effect: a state of the whole game.
func During(e effect.Effect) Cond { return guard{steps.GuardDuring(e)} }

// While holds while whose own entity, a plugin's, is under the effect.
func While(whose interface{ Entity() uid.UID64 }, e effect.Effect) Cond {
	return guard{steps.GuardWhile(whose.Entity, e)}
}

// Playing holds while the entity, or inside Here or Around the place turned to, plays the role.
func Playing(role *Part) Cond { return guard{steps.GuardPlaying(uint8(role.tag))} }

// Self holds for an entity carrying t. It is asked at the top of a rule alone.
func Self[F any](t tag.Tag[F]) Cond {
	return sides{self: plugin.SideOf(t), paired: true, mine: carrierOf(t)}
}

// Other holds for a pair whose other carries t, on a moment that is plugin.Met. It is asked at
// the top of a rule alone.
func Other[F any](t tag.Tag[F]) Cond {
	return sides{other: plugin.SideOf(t), paired: true, others: true}
}

// Carrying holds for an entity carrying the component T. It is asked at the top of a rule alone.
func Carrying[T any]() Cond {
	return sides{need: &having{state: func() state { return stateOf[T]() }}}
}

type guard struct{ g steps.Guard }

func (guard) cond() {}

type all struct{ conds []any }

func (all) cond() {}

type sides struct {
	self, other plugin.Side
	paired      bool
	others      bool
	mine        *side
	need        *having
}

func (sides) cond() {}

func (s sides) joined(o sides, name string) sides {
	if o.need != nil {
		if s.need != nil {
			panic(fmt.Sprintf("rule: %q: Carrying twice", name))
		}
		s.need = o.need
	}
	if o.paired {
		if o.mine != nil {
			if s.mine != nil {
				panic(fmt.Sprintf("rule: %q: Self twice", name))
			}
			s.self, s.mine = o.self, o.mine
		}
		if o.others {
			if s.others {
				panic(fmt.Sprintf("rule: %q: Other twice", name))
			}
			s.other, s.others = o.other, true
		}
		s.paired = true
	}
	return s
}

type asked[P any] struct {
	preds  []func(P) bool
	guards []steps.Guard
	sides  sides
	top    bool
	name   string
}

func (a *asked[P]) take(when any) {
	switch w := when.(type) {
	case nil:
	case func(P) bool:
		a.preds = append(a.preds, w)
	case guard:
		a.guards = append(a.guards, w.g)
	case all:
		for _, c := range w.conds {
			a.take(c)
		}
	case sides:
		if !a.top {
			panic(fmt.Sprintf("rule: %q: Self, Other and Carrying are asked at the top of a rule",
				a.name))
		}
		a.sides = a.sides.joined(w, a.name)
	default:
		if reflect.TypeOf(when).Kind() == reflect.Func {
			panic(fmt.Sprintf("rule: %q: %T is a condition on another moment or fact, not %v",
				a.name, when, reflect.TypeFor[P]()))
		}
		panic(fmt.Sprintf("rule: %q: %T is no condition", a.name, when))
	}
}
