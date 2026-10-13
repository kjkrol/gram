package gravity

import (
	"github.com/kjkrol/gram/rule/cond"
	"github.com/kjkrol/uid"
)

// Landing is a body coming down on what bears it, told once as it lands; Speed is how fast it
// fell, in world units a second.
type Landing struct {
	ID    uid.UID64
	Speed float64
}

// Who is the body landing.
func (l Landing) Who() uid.UID64 { return l.ID }

// HarderThan is the word of a body come down faster than speed (world units a second).
func HarderThan(speed float64) cond.Word[Landing] {
	return cond.NewWord("harder than", func(l Landing) bool { return l.Speed > speed })
}
