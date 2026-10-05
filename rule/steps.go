package rule

import (
	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/rule/effect"
)

// Then is a rule, named name: at every moment P a plugin's pass catches, for whom filter lets
// through, it runs step — On without a body, its steps made by the package's own functions (If,
// OneOf, Apply, Around…), the twins of a Moment's methods. A step a moment of P cannot run — a
// Here or an Around where P is not Placed, an If over another moment — panics by the rule's name.
func Then[P any](name string, filter Filter, step Step) Rule { return build[P](name, filter, step) }

// Not is the predicate holding where holds does not: If(Not(unit.Standing.Fallen), …).
func Not[P any](holds func(P) bool) func(P) bool { return func(p P) bool { return !holds(p) } }

// If runs step when holds says the moment holds, and fails otherwise; an If inside an If is both.
func If[P any](holds func(P) bool, step Step) Step { return steps.NewIf(holds, step) }

// OneOf runs its steps in order and does as the first that does not fail; it fails when all fail.
func OneOf(parts ...Step) Step { return steps.NewFirst("", parts...) }

// Steps runs its steps one after another while each does well; it fails with the first that fails.
func Steps(parts ...Step) Step { return steps.NewThen("", parts...) }

// Apply casts the effect on the entity, lasting as its Spec says.
func Apply(e effect.Effect) Step { return steps.NewApply(e) }

// Keep holds the effect on the entity as long as the rule keeps firing it.
func Keep(e effect.Effect) Step { return steps.NewKeep(e) }

// Dispel takes the effect off the entity with the effects' next pass, and does well.
func Dispel(e effect.Effect) Step { return steps.NewDispel(e) }

// Chance runs step with likelihood p, drawn afresh at every step of the game, and fails otherwise.
func Chance(p float64, step Step) Step { return steps.NewChance(p, step) }

// Unless runs step while the entity is not under the effect, and fails while it is.
func Unless(e effect.Effect, step Step) Step { return steps.NewUnless(e, step) }

// Under runs step while the entity is under the effect, and fails while it is not.
func Under(e effect.Effect, step Step) Step { return steps.NewUnder(e, step) }

// During runs step while the world is under the effect, and fails while it is not.
func During(e effect.Effect, step Step) Step { return steps.NewDuring(e, step) }

// OnWire runs step on the wire the entity is wired to, in place of the entity.
func OnWire(step Step) Step { return steps.NewOnWire(step) }

// WhileWire runs step while the wire the entity is wired to is under e.
func WhileWire(e effect.Effect, step Step) Step { return steps.NewWhileWire(e, step) }

// Playing runs step while the entity — inside Here or Around, the place turned to — plays role.
func Playing(role *Part, step Step) Step { return steps.NewPlaying(uint8(role.tag), step) }

// Order gives the command cmd for the entity each time it fires, and does well at once; one that
// is Aimed is told the moment's Subject, and fails while the moment names nobody.
func Order[C any](cmd C) Step { return steps.NewOrder(cmd) }

// ForOther runs step on each of the others the moment met, or on its Subject, in place of the
// entity.
func ForOther(step Step) Step { return steps.NewForOther(step) }

// Here runs step on each place the entity stands on, in place of the entity; the rule's moment
// must be Placed.
func Here(step Step) Step { return steps.NewAround(0, step) }

// Around runs step on each place within rings of where the entity stands, in place of the entity;
// the rule's moment must be Placed.
func Around(rings int, step Step) Step { return steps.NewAround(rings, step) }
