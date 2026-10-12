package tag_test

import (
	"testing"

	"github.com/kjkrol/gram/entity/tag"
)

type roles struct{}

const (
	hunter tag.Tag[roles] = iota
	hunted
)

func TestTags_WithAndWithout(t *testing.T) {
	var s tag.Tags[roles]
	s = s.With(hunter, hunted)
	if !s.Has(hunter) || !s.Has(hunted) {
		t.Fatalf("With set %b, want both bits", s)
	}
	s = s.Without(hunter)
	if s.Has(hunter) || !s.Has(hunted) {
		t.Errorf("Without left %b, want only hunted", s)
	}
}

func TestTag_InIsCarried(t *testing.T) {
	s := tag.Tags[roles](0).With(hunted)
	if !hunted.In(s) || hunter.In(s) {
		t.Errorf("In: hunted %v, hunter %v; want true and false", hunted.In(s), hunter.In(s))
	}
}
