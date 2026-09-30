package act

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/uid"
)

// Subject is a fact about another entity — whom it was struck by, who asked, who stands beside:
// under a When, On or If of such a fact, an Ask, a Relay or an Aimed action speaks to that entity;
// false for a fact about nobody just now.
type Subject interface{ Subject() (uid.UID64, bool) }

// MaxChain is how many entities an ask passes through at most, the first asker among them: a
// Relay past it, or back to one of them, is refused.
const MaxChain = 4

// AskLife is how long an ask or an answer waits to be taken up; older, it is dropped.
const AskLife = 3 * time.Second

// Chain is who an ask passed through before its asker, the first asker first.
type Chain struct {
	Who   [MaxChain]uid.UID64
	Links uint8
}

// Has reports whether id is on the chain.
func (c Chain) Has(id uid.UID64) bool {
	for _, w := range c.Who[:c.Links] {
		if w == id {
			return true
		}
	}
	return false
}

// with is c with id on its end; false when full.
func (c Chain) with(id uid.UID64) (Chain, bool) {
	if int(c.Links) == MaxChain {
		return c, false
	}
	c.Who[c.Links] = id
	c.Links++
	return c, true
}

// Asked is the fact of an ask for W: From who asks, Chain who it passed through before them, At
// when it came, on the world's clock. It stays until answered — Agree, Refuse — or AskLife is up.
type Asked[W any] struct {
	From  uid.UID64
	Chain Chain
	At    time.Duration
}

// Subject is who asks.
func (a Asked[W]) Subject() (uid.UID64, bool) { return a.From, true }

func (a Asked[W]) at() time.Duration { return a.At }

// Answer is what an asked entity answers.
type Answer uint8

const (
	// Agreed is a yes: it does what was asked.
	Agreed Answer = iota + 1
	// Refused is a no.
	Refused
	// Relayed is a wait: it asked another in turn, and answers once it can.
	Relayed
)

// Replied is the fact of an answer to the entity's ask for W: From who answers, Answer what, At
// when. An Ask takes it up; left, it is dropped after AskLife.
type Replied[W any] struct {
	From   uid.UID64
	Answer Answer
	At     time.Duration
}

// Subject is who answers.
func (r Replied[W]) Subject() (uid.UID64, bool) { return r.From, true }

func (r Replied[W]) at() time.Duration { return r.At }

// aged is a fact that is dropped once AskLife is up.
type aged interface{ at() time.Duration }

func newAsk[W any](name string, wait time.Duration, agreed Node, refused ...Node) Node {
	kids := []Node{agreed}
	kids = append(kids, refused...)
	return composite{name: name, kids: kids, sign: "ask[" + typeName[W]() + "]",
		make: func() step { return &ask[W]{wait: wait} }}
}

// The stages of an Ask, in its slot.
const (
	asking uint8 = iota
	agreedTo
	refusedBy
	passedOn
)

type ask[W any] struct {
	wait    time.Duration
	asked   *fact[Asked[W]]
	replied *fact[Replied[W]]
}

func (a *ask[W]) bind(s *system) {
	a.asked, a.replied = factOf[Asked[W]](s), factOf[Replied[W]](s)
}

func (a *ask[W]) enter(c *ctx, at int) {
	c.mind.Slot[at], c.mind.Since[at] = asking, c.now
	if !c.about || c.subject == c.id {
		c.mind.Slot[at] = refusedBy
		return
	}
	a.replied.drop(c) // an old answer is none to this ask
	if !a.asked.putOn(c, c.subject, Asked[W]{From: c.id, At: c.now}) {
		c.mind.Slot[at] = refusedBy // nobody there to ask
	}
}

func (a *ask[W]) halt(*ctx, int) {}

func (a *ask[W]) tick(c *ctx, at int, kids []int) Status {
	if stage := c.mind.Slot[at]; stage == asking || stage == passedOn {
		wait := a.wait
		if stage == passedOn {
			wait = AskLife
		}
		if r := a.replied.of(c); r != nil && r.At >= c.mind.Since[at] {
			a.replied.drop(c)
			switch r.Answer {
			case Agreed:
				c.mind.Slot[at] = agreedTo
			case Relayed:
				c.mind.Slot[at], c.mind.Since[at] = passedOn, c.now
			default:
				c.mind.Slot[at] = refusedBy
			}
		} else if c.now-c.mind.Since[at] >= wait {
			c.mind.Slot[at] = refusedBy
		}
	}
	switch c.mind.Slot[at] {
	case agreedTo:
		return c.run(kids[0])
	case refusedBy:
		if len(kids) > 1 {
			return c.run(kids[1])
		}
		return Failure
	}
	return Running
}

func newAgree[W any]() Node {
	return leaf{sign: "agree[" + typeName[W]() + "]", make: func() step { return &answer[W]{answer: Agreed} }}
}

func newRefuse[W any]() Node {
	return leaf{sign: "refuse[" + typeName[W]() + "]", make: func() step { return &answer[W]{answer: Refused} }}
}

type answer[W any] struct {
	basic
	answer  Answer
	asked   *fact[Asked[W]]
	replied *fact[Replied[W]]
}

func (a *answer[W]) bind(s *system) {
	a.asked, a.replied = factOf[Asked[W]](s), factOf[Replied[W]](s)
}

func (a *answer[W]) tick(c *ctx, _ int, _ []int) Status {
	q := a.asked.of(c)
	if q == nil {
		return Failure
	}
	a.replied.putOn(c, q.From, Replied[W]{From: c.id, Answer: a.answer, At: c.now})
	a.asked.drop(c)
	return Success
}

func newRelay[W any]() Node {
	return leaf{sign: "relay[" + typeName[W]() + "]", make: func() step { return &relay[W]{} }}
}

type relay[W any] struct {
	asked   *fact[Asked[W]]
	replied *fact[Replied[W]]
}

func (r *relay[W]) bind(s *system) {
	r.asked, r.replied = factOf[Asked[W]](s), factOf[Replied[W]](s)
}

func (r *relay[W]) enter(c *ctx, at int) {
	c.mind.Slot[at], c.mind.Since[at] = 0, c.now
	q := r.asked.of(c)
	to := c.subject
	if q == nil || !c.about || to == c.id || to == q.From || q.Chain.Has(to) {
		return
	}
	chain, ok := q.Chain.with(q.From)
	if !ok || !r.asked.putOn(c, to, Asked[W]{From: c.id, Chain: chain, At: c.now}) {
		return
	}
	r.replied.drop(c) // an old answer is none to this ask
	r.replied.putOn(c, q.From, Replied[W]{From: c.id, Answer: Relayed, At: c.now})
	c.mind.Slot[at] = 1
}

func (r *relay[W]) halt(*ctx, int) {}

func (r *relay[W]) tick(c *ctx, at int, _ []int) Status {
	if c.mind.Slot[at] == 0 {
		return Failure
	}
	if a := r.replied.of(c); a != nil && a.At >= c.mind.Since[at] {
		r.replied.drop(c)
		switch a.Answer {
		case Refused:
			return Failure
		default: // passed further on, or making way further on: the asker waits again
			if q := r.asked.of(c); q != nil {
				r.replied.putOn(c, q.From, Replied[W]{From: c.id, Answer: Relayed, At: c.now})
			}
		}
	}
	return Running
}

// conversed is the components of an ask for W, for the saves: the asks and the answers.
func conversed[W any]() []goke.CompToken {
	return []goke.CompToken{goke.LoadComp[Asked[W]](), goke.LoadComp[Replied[W]]()}
}

func (*ask[W]) loads() []goke.CompToken    { return conversed[W]() }
func (*answer[W]) loads() []goke.CompToken { return conversed[W]() }
func (*relay[W]) loads() []goke.CompToken  { return conversed[W]() }
