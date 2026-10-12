package rule

import (
	"time"

	"github.com/kjkrol/gram/internal/steps"
)

// Ordered is the step of an Order, which a plan may follow with Until or Stay.
type Ordered struct{ Step }

// Until is the command, then waiting until the entity is given the fact F afresh, holding as
// holds says with one.
func (o Ordered) Until[F any](holds ...func(F) bool) Step {
	return step{steps.NewThen("", o.node(), steps.NewUntil(holds...))}
}

// Stay is the command given once, then staying for as long as the branch runs.
func (o Ordered) Stay() Step { return step{steps.NewThen("", o.node(), steps.NewIdle())} }

// Until runs until the entity is given the fact F afresh, holding as holds says with one; a
// fact carried from before does not count.
func Until[F any](holds ...func(F) bool) Step { return step{steps.NewUntil(holds...)} }

// Wait runs for d, then does well.
func Wait(d time.Duration) Step { return step{steps.NewWait(d)} }

// Timeout runs s for at most d, failing and stopping it when d is up.
func Timeout(d time.Duration, s Step) Step { return step{steps.NewTimeout(d, s.node())} }

// Cooldown runs s, and once it is done, fails for d before it runs it again.
func Cooldown(d time.Duration, s Step) Step { return step{steps.NewCooldown(d, s.node())} }

// Idle runs for ever and does nothing.
func Idle() Step { return step{steps.NewIdle()} }

// Invert does well where s fails, and fails where it does well.
func Invert(s Step) Step { return step{steps.NewInvert(s.node())} }

// Asked is the fact of an ask for W on the entity asked, which a Branch on it answers with
// Agree, Refuse or Relay.
type Asked[W any] = steps.Asked[W]

// Replied is the fact of an answer to the entity's ask for W.
type Replied[W any] = steps.Replied[W]

// Answer is what an asked entity answers: Agreed, Refused or Relayed (passed on, wait).
type Answer = steps.Answer

const (
	Agreed  = steps.Agreed
	Refused = steps.Refused
	Relayed = steps.Relayed
)

// MaxChain is how many entities an ask passes through at most, AskLife how long an ask or an
// answer waits to be taken up.
const (
	MaxChain = steps.MaxChain
	AskLife  = steps.AskLife
)

// Ask asks the subject of the fact it stands under for W and waits up to wait for the answer: a
// yes runs agreed, a no, no answer in time or no subject with a mind runs refused (fails without
// one).
func Ask[W any](name string, wait time.Duration, agreed Step, refused ...Step) Step {
	return step{steps.NewAsk[W](name, wait, agreed.node(), nodes(refused)...)}
}

// Agree answers the entity's ask for W yes, and does well; it fails with no ask.
func Agree[W any]() Step { return step{steps.NewAgree[W]()} }

// Refuse answers the entity's ask for W no, and does well; it fails with no ask.
func Refuse[W any]() Step { return step{steps.NewRefuse[W]()} }

// Relay passes the entity's ask for W on to the subject of the fact it stands under and tells
// the asker to wait. It fails when the one asked on refuses, or at once with no ask, no subject,
// one already on the chain, the asker or itself, or the chain full.
func Relay[W any]() Step { return step{steps.NewRelay[W]()} }
