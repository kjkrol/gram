package rule

import (
	"fmt"

	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/rule/internal/engine"
	"github.com/kjkrol/uid"
)

// About is a moment of one entity: whose it is. A host's payload — a unit.Standing, a
// vision.Sighting, a collision.Struck, a clock.Moment (the clock's own entity) — is one; on a moment
// of no entity steps acting on one fail.
type About interface{ Who() uid.UID64 }

// Met is a moment of one entity with others — whom it saw, whom it struck: the host matches it in
// pairs, and ForOther turns a step on the others.
type Met interface {
	About
	Whom(each func(uid.UID64))
}

// Placed is a moment of an entity standing somewhere, on places that are entities of their own —
// a board's cells — whose host tells, in the Tick, which places lie round it (Tick.Around):
// Here and Around turn a step on them. The moment is data alone.
type Placed interface {
	About
	Placed()
}

// Subject is a moment or a fact about another entity — whom it was struck by, who asked, whom it
// touched: an Aimed command given on it is about that entity; false for nobody just now, and an
// Aimed command given then fails.
type Subject interface{ Subject() (uid.UID64, bool) }

// Aimed is a command told whom it is about — the subject of the moment or the fact it is given
// on — as it is issued: whose way to step off, whose goal to swap with.
type Aimed interface{ Aim(who uid.UID64) }

// Step is a part of a rule or a plan, made by the methods of a Moment or of a plan's Actor — or by
// a plugin's own functions, built on them.
type Step = engine.Step

// Moment is the moment P a rule is written for: its methods make the steps the rule takes then,
// each done within the plugin's pass. On hands one to the rule's body; a function writing part of
// a rule takes it as its own.
type Moment[P any] struct{}

// OneOf runs its steps in order and does as the first that does not fail; it fails when all fail.
func (m *Moment[P]) OneOf(steps ...Step) Step { return engine.NewFirst("", steps...) }

// Steps runs its steps one after another while each does well; it fails with the first that fails.
func (m *Moment[P]) Steps(steps ...Step) Step { return engine.NewThen("", steps...) }

// If runs step when holds says the moment holds, and fails otherwise.
func (m *Moment[P]) If(holds func(P) bool, step Step) Step { return engine.NewIf(holds, step) }

// Not does well where step fails, and fails where it does well.
func (m *Moment[P]) Not(step Step) Step { return engine.NewInvert(step) }

// Apply casts the effect on the entity, lasting as its Spec says.
func (m *Moment[P]) Apply(e effect.Effect) Step { return engine.NewApply(e) }

// Keep holds the effect on the entity as long as the rule keeps firing it: cast for two ticks at
// a time, it ends by itself when the rule stops.
func (m *Moment[P]) Keep(e effect.Effect) Step { return engine.NewKeep(e) }

// Dispel takes the effect off the entity with the effects' next pass, and does well; a rule
// keeping it may cast it again.
func (m *Moment[P]) Dispel(e effect.Effect) Step { return engine.NewDispel(e) }

// Chance runs step with likelihood p, drawn afresh at every step of the game from the world's seed,
// the moment's game time and the entity, and fails otherwise: the same after a load and in a replay.
func (m *Moment[P]) Chance(p float64, step Step) Step { return engine.NewChance(p, step) }

// Unless runs step while the entity is not under the effect, and fails while it is: "at most once
// a while" is Unless an effect lasting that while, applied in step.
func (m *Moment[P]) Unless(e effect.Effect, step Step) Step { return engine.NewUnless(e, step) }

// Under runs step while the entity is under the effect, and fails while it is not.
func (m *Moment[P]) Under(e effect.Effect, step Step) Step { return engine.NewUnder(e, step) }

// During runs step while the world is under the effect — a state of the whole game, a lever
// pulled, an alarm (world.Apply) — and fails while it is not.
func (m *Moment[P]) During(e effect.Effect, step Step) Step { return engine.NewDuring(e, step) }

// Order gives the command cmd for the entity each time it fires — the same command a player gives
// — and does well at once; one that is Aimed is told the moment's Subject, and fails while the
// moment names nobody.
func (m *Moment[P]) Order[C any](cmd C) Step { return engine.NewOrder(cmd) }

// ForOther runs step on each of the others the moment met — whom the entity saw, whom it struck —
// in place of the entity: an effect applied there, a command ordered for it.
func (m *Moment[P]) ForOther(step Step) Step {
	return engine.NewForOther(step)
}

// Here runs step on each place the entity stands on — the cells under it — in place of the entity:
// an effect applied to the ground. The moment must be Placed.
func (m *Moment[P]) Here(step Step) Step { return m.Around(0, step) }

// Around runs step on each place within rings of where the entity stands, those it stands on
// among them, in place of the entity. The moment must be Placed.
func (m *Moment[P]) Around(rings int, step Step) Step {
	if _, ok := any(*new(P)).(Placed); !ok {
		panic(fmt.Sprintf("rule: Here and Around need a moment that is Placed, not %T", *new(P)))
	}
	return engine.NewAround(rings, step)
}
