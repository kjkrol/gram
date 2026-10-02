package rule

import (
	"fmt"

	"github.com/kjkrol/gram/internal/steps"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/rule/effect"
)

// Step is a part of a rule or a plan, made by the methods of a Moment or of a plan's Actor — or by
// a plugin's own functions, built on them.
type Step = steps.Step

// Moment is the moment P a rule is written for: its methods make the steps the rule takes then,
// each done within the plugin's pass. On hands one to the rule's body; a function writing part of
// a rule takes it as its own.
type Moment[P any] struct{}

// OneOf runs its steps in order and does as the first that does not fail; it fails when all fail.
func (m *Moment[P]) OneOf(parts ...Step) Step { return steps.NewFirst("", parts...) }

// Steps runs its steps one after another while each does well; it fails with the first that fails.
func (m *Moment[P]) Steps(parts ...Step) Step { return steps.NewThen("", parts...) }

// If runs step when holds says the moment holds, and fails otherwise.
func (m *Moment[P]) If(holds func(P) bool, step Step) Step { return steps.NewIf(holds, step) }

// Not does well where step fails, and fails where it does well.
func (m *Moment[P]) Not(step Step) Step { return steps.NewInvert(step) }

// Apply casts the effect on the entity, lasting as its Spec says.
func (m *Moment[P]) Apply(e effect.Effect) Step { return steps.NewApply(e) }

// Keep holds the effect on the entity as long as the rule keeps firing it: cast for two ticks at
// a time, it ends by itself when the rule stops.
func (m *Moment[P]) Keep(e effect.Effect) Step { return steps.NewKeep(e) }

// Dispel takes the effect off the entity with the effects' next pass, and does well; a rule
// keeping it may cast it again.
func (m *Moment[P]) Dispel(e effect.Effect) Step { return steps.NewDispel(e) }

// Chance runs step with likelihood p, drawn afresh at every step of the game from the world's seed,
// the moment's game time and the entity, and fails otherwise: the same after a load and in a replay.
func (m *Moment[P]) Chance(p float64, step Step) Step { return steps.NewChance(p, step) }

// Unless runs step while the entity is not under the effect, and fails while it is: "at most once
// a while" is Unless an effect lasting that while, applied in step.
func (m *Moment[P]) Unless(e effect.Effect, step Step) Step { return steps.NewUnless(e, step) }

// Under runs step while the entity is under the effect, and fails while it is not.
func (m *Moment[P]) Under(e effect.Effect, step Step) Step { return steps.NewUnder(e, step) }

// During runs step while the world is under the effect — a state of the whole game, a lever
// pulled, an alarm (world.Apply) — and fails while it is not.
func (m *Moment[P]) During(e effect.Effect, step Step) Step { return steps.NewDuring(e, step) }

// OnWire runs step on the wire the entity is wired to (Wired), in place of the entity — an effect
// applied there drives the wire — and fails for one wired to none.
func (m *Moment[P]) OnWire(step Step) Step { return steps.NewOnWire(step) }

// WhileWire runs step while the wire the entity is wired to is under e — a trapdoor kept open
// while its lever's wire is on — and fails while it is not, or for one wired to none.
func (m *Moment[P]) WhileWire(e effect.Effect, step Step) Step { return steps.NewWhileWire(e, step) }

// Playing runs step while the entity plays role, and fails while it does not: inside Here or
// Around, the place it turned to — a lever beside a unit, among the trapdoors on its wire.
func (m *Moment[P]) Playing(role *Part, step Step) Step {
	return steps.NewPlaying(uint8(role.tag), step)
}

// Order gives the command cmd for the entity each time it fires — the same command a player gives
// — and does well at once; one that is Aimed is told the moment's Subject, and fails while the
// moment names nobody.
func (m *Moment[P]) Order[C any](cmd C) Step { return steps.NewOrder(cmd) }

// ForOther runs step on each of the others the moment met — whom the entity saw, whom it struck —
// in place of the entity: an effect applied there, a command ordered for it.
func (m *Moment[P]) ForOther(step Step) Step {
	return steps.NewForOther(step)
}

// Here runs step on each place the entity stands on — the cells under it — in place of the entity:
// an effect applied to the ground. The moment must be Placed.
func (m *Moment[P]) Here(step Step) Step { return m.Around(0, step) }

// Around runs step on each place within rings of where the entity stands, those it stands on
// among them, in place of the entity. The moment must be Placed.
func (m *Moment[P]) Around(rings int, step Step) Step {
	if _, ok := any(*new(P)).(plugin.Placed); !ok {
		panic(fmt.Sprintf("rule: Here and Around need a moment that is Placed, not %T", *new(P)))
	}
	return steps.NewAround(rings, step)
}
