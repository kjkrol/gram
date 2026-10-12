package rule

import (
	"time"

	"github.com/kjkrol/gram/internal/steps"
)

// Until runs until the entity is given the fact F afresh, holding as holds says with one; a
// fact carried from before does not count.
func Until[F any](holds ...func(F) bool) Step { return step{steps.NewUntil(holds...)} }

// Wait runs for d, then does well.
func Wait(d time.Duration) Step { return step{steps.NewWait(d)} }

// Idle runs for ever and does nothing.
func Idle() Step { return step{steps.NewIdle()} }

// From is what a message carries of its sender. A command embedding it, given with Apply on a
// branch whose On is Other, lands on that entity as a fact in place of going to a queue, the
// engine filling in who sent it; one without a plan takes none. A branch on the command's type
// answers it, Apply on Other again.
type From = steps.From

// MaxChain is how many times a message of one type may be passed on, AskLife how long a message
// waits to be taken up before it is dropped.
const (
	MaxChain = steps.MaxChain
	AskLife  = steps.AskLife
)
