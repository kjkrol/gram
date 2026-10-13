package cond

import (
	"reflect"

	"github.com/kjkrol/gram/internal/steps"
)

// Word is a word of a moment or a fact P: a state Self or Other is in as the moment says
// (unit.Fallen, vision.SeesNobody); a plugin makes its own with NewWord, a game those of its
// own facts.
type Word[P any] struct {
	name  string
	other bool
	holds func(P) bool
}

// NewWord is the word named name of the moment P, of the entity the moment is about, holding
// where holds does.
func NewWord[P any](name string, holds func(P) bool) Word[P] {
	return Word[P]{name: name, holds: holds}
}

// OfOther is the word of the other the moment met, in Other alone.
func (w Word[P]) OfOther() Word[P] { w.other = true; return w }

// Holds reports whether the word holds of p.
func (w Word[P]) Holds(p P) bool { return w.holds(p) }

func (w Word[P]) ofOther() bool { return w.other }
func (w Word[P]) named() string { return w.name }
func (w Word[P]) side() string {
	if w.other {
		return "other"
	}
	return "entity"
}

func (w Word[P]) lower(of steps.Whose) (steps.Guard, reflect.Type) {
	return steps.GuardWord(w.name, w.holds, of), reflect.TypeFor[P]()
}

type word interface {
	ofOther() bool
	named() string
	side() string
	lower(of steps.Whose) (steps.Guard, reflect.Type)
}
