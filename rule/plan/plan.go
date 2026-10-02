package plan

import (
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/internal/engine"
)

// New is the plan named name — what a save knows it by — whose steps body writes for an Actor: a
// component a kind gives its entities, which behave by it from then on. One name is one plan:
// written again, it must be alike.
func New(name string, body func(a *Actor) rule.Step) comp.Comp {
	return comp.Const(Mind{Plan: engine.Register(name, body(&Actor{}))})
}

// Mind is the state of an entity's plan: which plan, the steps running since the last tick and
// what each keeps. A kind gives it with New; the plans' system runs it.
type Mind = engine.Mind

// AskLife is how long an ask or an answer waits to be taken up; MaxChain how many entities an ask
// passes through at most, the first asker among them.
const (
	AskLife  = engine.AskLife
	MaxChain = engine.MaxChain
)

// Asked is the fact of an ask for W on the one asked — From who asks, Chain who it passed through
// before them, At when it came — until it is answered or AskLife is up; Replied is the asker's fact
// of the Answer; Chain who an ask passed through.
type (
	Asked[W any]   = engine.Asked[W]
	Replied[W any] = engine.Replied[W]
	Chain          = engine.Chain
	Answer         = engine.Answer
)

// What an asked entity answers: Agreed, a yes; Refused, a no; Relayed, a wait — it asked another
// in turn and answers once it can.
const (
	Agreed  = engine.Agreed
	Refused = engine.Refused
	Relayed = engine.Relayed
)
