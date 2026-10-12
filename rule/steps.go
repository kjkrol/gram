package rule

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/rule/effect"
)

// Step is a part of a rule or a plan: a Branch or what the package's functions make. The steps
// that last over ticks are a plan's alone; a rule is refused one as it is made.
type Step interface{ node() steps.Step }

type step struct{ s steps.Step }

func (s step) node() steps.Step { return s.s }

func nodes(parts []Step) []steps.Step {
	out := make([]steps.Step, len(parts))
	for i, p := range parts {
		out[i] = p.node()
	}
	return out
}

// OneOf does as the first of its steps that does not fail, and fails when all fail. In a plan an
// earlier step, once it can, takes over from a later one.
func OneOf(parts ...Step) Step { return step{steps.NewFirst("", nodes(parts)...)} }

// Steps runs its steps one after another while each does well, and fails with the first that
// fails.
func Steps(parts ...Step) Step { return step{steps.NewThen("", nodes(parts)...)} }

// Apply casts the effect on the entity, lasting as its Spec says, and does well.
func Apply(e effect.Effect) Step { return step{steps.NewApply(e)} }

// Keep holds the effect on the entity as long as the rule keeps firing it, or in a plan as long
// as its branch runs; it fails when someone else takes the effect off.
func Keep(e effect.Effect) Step { return step{steps.NewKeep(e)} }

// Dispel takes the effect off the entity with the effects' next pass, and does well.
func Dispel(e effect.Effect) Step { return step{steps.NewDispel(e)} }

// Chance runs s with likelihood p, drawn from the world's seed, the game time and the entity,
// and fails otherwise.
func Chance(p float64, s Step) Step { return step{steps.NewChance(p, s.node())} }

// Either runs a or b, as likely one as the other, drawn as Chance draws.
func Either(a, b Step) Step { return step{steps.NewEither(a.node(), b.node())} }

// Once runs s and casts e, and fails while the entity is under e.
func Once(e effect.Effect, s Step) Step { return step{steps.NewOnce(e, s.node())} }

// Order gives cmd for the entity and does well at once. A plugin.Aimed command is told the
// moment's or the fact's Subject, and fails while there is none.
func Order[C control.Command](cmd C) Ordered { return Ordered{step{steps.NewOrder(cmd)}} }

// ForOther runs s on each of the others the moment met, or on its Subject, in place of the
// entity.
func ForOther(s Step) Step { return step{steps.NewForOther(s.node())} }

// Here runs s on each place the entity stands on, in place of the entity; the moment must be
// plugin.Placed.
func Here(s Step) Step { return step{steps.NewAround(0, s.node())} }

// Around runs s on each place within rings of where the entity stands, in place of the entity;
// the moment must be plugin.Placed.
func Around(rings int, s Step) Step { return step{steps.NewAround(rings, s.node())} }
