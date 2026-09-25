package game

import (
	"testing"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/render"
)

type stubScene struct {
	name      string
	focusable bool
}

var _ Scene = (*stubScene)(nil)

func (s *stubScene) Name() string           { return s.name }
func (s *stubScene) Layers() []render.Layer { return nil }
func (s *stubScene) HandleEvents(*control.InputEvents, Runtime, Composition) {
}
func (s *stubScene) Focusable() bool { return s.focusable }

func TestNewStack_GetAndAll(t *testing.T) {
	a := &stubScene{name: "a", focusable: true}
	b := &stubScene{name: "b", focusable: true}

	s, err := NewStack(a, b)
	if err != nil {
		t.Fatalf("NewStack: unexpected error: %v", err)
	}
	if got, ok := s.Get("a"); !ok || got != a {
		t.Errorf("Get(%q) = %v, %v, want %v, true", "a", got, ok, a)
	}
	if got, ok := s.Get("missing"); ok {
		t.Errorf("Get(%q) = %v, true, want ok=false", "missing", got)
	}
	if all := s.All(); len(all) != 2 {
		t.Errorf("All() = %v, want 2 scenes", all)
	}
}

func TestNewStack_RejectsDuplicateName(t *testing.T) {
	a := &stubScene{name: "dup"}
	b := &stubScene{name: "dup"}

	if _, err := NewStack(a, b); err == nil {
		t.Fatal("NewStack: expected error for duplicate scene name, got nil")
	}
}
