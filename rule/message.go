package rule

import "github.com/kjkrol/gram/internal/steps"

// From is what a message carries of its sender. A command embedding it, given with
// plan.Other.Do, lands on that entity as a fact in place of going to a queue, the engine
// filling in who sent it; one without a plan takes none. A loop of the command's type answers
// it, plan.Other.Do again.
type From = steps.From

// MaxChain is how many times a message of one type may be passed on, AskLife how long a message
// waits to be taken up before it is dropped.
const (
	MaxChain = steps.MaxChain
	AskLife  = steps.AskLife
)
