package effect

import "time"

// MaxEffects is how many effects one entity holds at once.
const MaxEffects = 8

// ID names one defined effect; Define hands them out by call order.
type ID uint8

// Forever is the Left of a slot that lasts until Dispel.
const Forever = time.Duration(-1)

// Active is what an entity is under: up to MaxEffects slots, each an effect and its time left.
// Saved with the entity, so a load resumes the countdown.
type Active struct {
	Slots [MaxEffects]Slot
}

// Slot is one effect on an entity: which, how long it has left (Forever for no limit), whether it
// has begun — its changes are applied on the tick after Cast — and whether it was dispelled, so its
// Then is not cast.
type Slot struct {
	Kind      ID
	Left      time.Duration
	State     SlotState
	Dispelled bool
}

// SlotState is where a Slot is in its life.
type SlotState uint8

const (
	Empty   SlotState = iota
	Pending           // cast, not yet applied
	Running
)

// Has reports whether effect is on the entity, pending or running.
func (a *Active) Has(effect ID) bool {
	for _, s := range a.Slots {
		if s.State != Empty && s.Kind == effect {
			return true
		}
	}
	return false
}

// slot finds the first live slot of effect, or -1.
func (a *Active) slot(effect ID) int {
	for i, s := range a.Slots {
		if s.State != Empty && s.Kind == effect {
			return i
		}
	}
	return -1
}

// free finds an empty slot, or -1.
func (a *Active) free() int {
	for i, s := range a.Slots {
		if s.State == Empty {
			return i
		}
	}
	return -1
}

// empty reports no live slot at all.
func (a *Active) empty() bool {
	for _, s := range a.Slots {
		if s.State != Empty {
			return false
		}
	}
	return true
}
