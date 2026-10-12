package gravity

import "github.com/kjkrol/uid"

// Landing is a body coming down on what bears it, told once as it lands; Speed is how fast it
// fell, in world units a second.
type Landing struct {
	ID    uid.UID64
	Speed float64
}

// Who is the body landing.
func (l Landing) Who() uid.UID64 { return l.ID }

// HarderThan is the condition that the body came down faster than speed (world units a second):
// a rule's When on a Landing.
func HarderThan(speed float64) func(Landing) bool {
	return func(l Landing) bool { return l.Speed > speed }
}
