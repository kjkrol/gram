package effect

import "time"

// maxEffects is how many effects one entity holds at once.
const maxEffects = 8

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

// Has reports whether effect is on the entity, pending or running.
func (a *Active) has(effect effectID) bool {
	for _, s := range a.Slots {
		if s.State != slotEmpty && s.Kind == effect {
			return true
		}
	}
	return false
}

// slot finds the first live slot of effect, or -1.
func (a *Active) slot(effect effectID) int {
	for i, s := range a.Slots {
		if s.State != slotEmpty && s.Kind == effect {
			return i
		}
	}
	return -1
}

// free finds an empty slot, or -1.
func (a *Active) free() int {
	for i, s := range a.Slots {
		if s.State == slotEmpty {
			return i
		}
	}
	return -1
}

// empty reports no live slot at all.
func (a *Active) empty() bool {
	for _, s := range a.Slots {
		if s.State != slotEmpty {
			return false
		}
	}
	return true
}
