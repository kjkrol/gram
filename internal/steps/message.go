package steps

import (
	"time"

	"github.com/kjkrol/uid"
)

// Subject is a fact or a moment about another entity, whom an Aimed command or a message
// under it goes to; false for nobody just now.
type Subject interface{ Subject() (uid.UID64, bool) }

// MaxChain is how many times a message of one type may be passed on, the first sender's
// counted.
const MaxChain = 4

// AskLife is how long a message waits to be taken up before it is dropped.
const AskLife = 3 * time.Second

// From is what a message carries of its sender: a command embedding it, given for the other
// of a fact, lands on that entity as a fact in place of going to a queue. The engine fills it.
type From struct {
	By   uid.UID64
	At   time.Duration // on the world's clock
	Hops uint8         // how many times the message was passed on
}

// Subject is the sender.
func (f From) Subject() (uid.UID64, bool) { return f.By, true }

func (f From) at() time.Duration { return f.At }
func (f From) hops() uint8       { return f.Hops }
func (f *From) from() *From      { return f }

type message interface{ from() *From }

type aged interface{ at() time.Duration }

type hopped interface{ hops() uint8 }
