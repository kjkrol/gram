package rule

import (
	"fmt"
	"reflect"

	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Cond is a condition a Branch asks besides what holds of its moment or fact: Under, Playing,
// Chance, a moment's own word. A predicate of the moment or fact, func(P) bool, stands beside
// them in When as it is.
type Cond interface{ cond() }

// All holds where every one of conds holds: predicates and Conds alike.
func All(conds ...any) Cond { return all{conds} }

// Not holds where c, a predicate or a Cond, does not.
func Not(c any) Cond { return negated{c} }

// Under holds while the entity is under the effect; On asks it of another: entity.World for
// the whole game, a plugin for its own entity.
func Under(e effect.Effect) UnderCond { return UnderCond{e: e} }

// UnderCond is the condition Under makes.
type UnderCond struct {
	e    effect.Effect
	whom any
}

func (UnderCond) cond() {}

// On asks whether whom is under the effect in place of the entity: entity.World, or a plugin
// (anything with an Entity).
func (u UnderCond) On(whom any) Cond {
	switch w := whom.(type) {
	case entity.Whom:
		if !w.IsWorld() {
			panic(fmt.Sprintf("rule: Under(…).On(%v): only entity.World or a plugin", w))
		}
	case interface{ Entity() uid.UID64 }:
	default:
		panic(fmt.Sprintf("rule: Under(…).On(%T): only entity.World or a plugin", whom))
	}
	u.whom = whom
	return u
}

func (u UnderCond) guard() steps.Guard {
	switch w := u.whom.(type) {
	case nil:
		return steps.GuardUnder(u.e)
	case entity.Whom:
		return steps.GuardDuring(u.e)
	case interface{ Entity() uid.UID64 }:
		return steps.GuardWhile(w.Entity, u.e)
	}
	panic("unreachable")
}

// Playing holds while the entity, or the place On turned to, plays the role.
func Playing(role *Part) Cond { return playing{role} }

type playing struct{ role *Part }

func (playing) cond() {}

// Chance holds with the likelihood p, drawn from the world's seed, the game time and the
// entity, so a load and a replay draw alike.
func Chance(p float64) Cond { return guard{steps.GuardChance(p)} }

// Paired holds for a pair whose other plays the role, on a moment that is plugin.Met; it is
// the word a plugin's own are made of (vision.Seeing, collision.Met), asked at the top of a
// rule alone.
func Paired(role *Part) Cond { return sides{other: plugin.SideOf(role.tag), paired: true} }

type guard struct{ g steps.Guard }

func (guard) cond() {}

type all struct{ conds []any }

func (all) cond() {}

type negated struct{ c any }

func (negated) cond() {}

type sides struct {
	self   plugin.Side // the first Playing at the top of a pair rule
	other  plugin.Side
	paired bool
}

func (sides) cond() {}

func (s sides) joined(o sides, name string) sides {
	if s.paired {
		panic(fmt.Sprintf("rule: %q: the other's role twice", name))
	}
	return o
}

// asked is a Branch's When taken apart: what holds of its P, what its steps ask as they run,
// and whom its host lets through.
type asked[P any] struct {
	preds  []func(P) bool
	guards []steps.Guard
	sides  sides
	roles  []*Part // Playing at the top of a rule: its host lets the players through
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
	case playing:
		if a.top {
			a.roles = append(a.roles, w.role)
		} else {
			a.guards = append(a.guards, w.guard())
		}
	case UnderCond:
		a.guards = append(a.guards, w.guard())
	case negated:
		a.takeNot(w.c)
	case all:
		for _, c := range w.conds {
			a.take(c)
		}
	case sides:
		if !a.top {
			panic(fmt.Sprintf("rule: %q: the other's role is asked at the top of a rule", a.name))
		}
		a.sides = a.sides.joined(w, a.name)
	default:
		a.refuse(when)
	}
}

func (a *asked[P]) takeNot(c any) {
	switch w := c.(type) {
	case func(P) bool:
		a.preds = append(a.preds, func(p P) bool { return !w(p) })
	case guard:
		a.guards = append(a.guards, steps.GuardNot(w.g))
	case playing:
		a.guards = append(a.guards, steps.GuardNot(w.guard()))
	case UnderCond:
		a.guards = append(a.guards, steps.GuardNot(w.guard()))
	default:
		a.refuse(c)
	}
}

func (p playing) guard() steps.Guard { return steps.GuardPlaying(uint8(p.role.tag)) }

func (a *asked[P]) refuse(when any) {
	if reflect.TypeOf(when).Kind() == reflect.Func {
		panic(fmt.Sprintf("rule: %q: %T is a condition on another moment or fact, not %v",
			a.name, when, reflect.TypeFor[P]()))
	}
	panic(fmt.Sprintf("rule: %q: %T is no condition", a.name, when))
}
