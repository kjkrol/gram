package view

import (
	"testing"

	"github.com/kjkrol/uid"
)

func TestEntitySet_AddHasClear(t *testing.T) {
	var s EntitySet
	if s.Has(uid.UID64(5)) || s.Len() != 0 {
		t.Fatal("a zero EntitySet holds something")
	}
	s.Add(uid.UID64(5))
	s.Add(uid.UID64(70)) // a second word
	if !s.Has(uid.UID64(5)) || !s.Has(uid.UID64(70)) || s.Has(uid.UID64(6)) {
		t.Errorf("Has after Add: 5=%v 70=%v 6=%v, want true true false", s.Has(5), s.Has(70), s.Has(6))
	}
	if s.Len() != 2 {
		t.Errorf("Len = %d, want 2", s.Len())
	}
	if s.Has(uid.UID64(1 << 20)) {
		t.Error("an index far past the set's size is reported present")
	}
	s.Clear()
	if s.Has(uid.UID64(5)) || s.Len() != 0 {
		t.Error("Clear left something in the set")
	}
}
