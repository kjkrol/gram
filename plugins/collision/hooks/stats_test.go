package hooks_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/collision/hooks"
)

func TestCountContacts_CountsEachContactOnce(t *testing.T) {
	var s hooks.ContactStats

	colliding(t, false, 1, hooks.CountContacts(&s))

	if s.Counter != 1 {
		t.Errorf("Counter = %d, want 1 for one pair in contact", s.Counter)
	}
}

func TestCountContacts_AccumulatesAcrossTicks(t *testing.T) {
	s := hooks.ContactStats{Counter: 5}

	colliding(t, false, 3, hooks.CountContacts(&s))

	if s.Counter != 8 {
		t.Errorf("Counter = %d, want 8 — three more ticks in contact on top of the 5 it started with", s.Counter)
	}
}
