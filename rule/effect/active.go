package effect

import (
	"time"

	"github.com/kjkrol/gram/entity/tag"
)

// maxEffects is how many effects one entity holds at once.
const maxEffects = 8

// wideEffects is how many effects a Wide holds at once: as many as a game may define.
const wideEffects = tag.MaxTagsPerFamily - 1

// effectID names one defined effect; Define hands them out by call order.
type effectID uint8

// Forever is the Left of a slot that lasts until Dispel.
const Forever = time.Duration(-1)

// Active is what an entity is under: up to maxEffects slots, each an effect and its time left.
// savedValue with the entity, so a load resumes the countdown.
type Active struct {
	Slots [maxEffects]effectSlot
}

// effectSlot is one effect on an entity: which, how long it has left (Forever for no limit), whether it
// has begun — its changes are applied on the tick after Cast — and whether it was dispelled, so its
// Then is not cast.
type effectSlot struct {
	Kind      effectID
	Left      time.Duration
	State     slotState
	Dispelled bool
}

// slotState is where a effectSlot is in its life.
type slotState uint8

const (
	slotEmpty   slotState = iota
	slotPending           // cast, not yet applied
	slotRunning
)

// Wide is an Active with a slot for every effect a game may define: what a plugin's own entity is
// under (world.Self), so the states of the sky or of the whole game are not held to maxEffects at
// once. Saved with the entity.
type Wide struct {
	Slots [wideEffects]effectSlot
}

// has reports whether effect is in slots, pending or running.
func has(slots []effectSlot, effect effectID) bool { return slotOf(slots, effect) >= 0 }

// slotOf finds the first live slot of effect, or -1.
func slotOf(slots []effectSlot, effect effectID) int {
	for i, s := range slots {
		if s.State != slotEmpty && s.Kind == effect {
			return i
		}
	}
	return -1
}

// free finds an empty slot, or -1.
func free(slots []effectSlot) int {
	for i, s := range slots {
		if s.State == slotEmpty {
			return i
		}
	}
	return -1
}

// empty reports no live slot at all.
func empty(slots []effectSlot) bool {
	for _, s := range slots {
		if s.State != slotEmpty {
			return false
		}
	}
	return true
}
