package steps

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Guard is a condition on the entity's or the world's state a branch asks besides its fact or
// moment.
type Guard interface {
	holds(c *ctx) bool
	sign() string
}

// GuardUnder holds while the entity is under e.
func GuardUnder(e effect.Effect) Guard { return underGuard{e: e} }

// GuardFree holds while the entity is not under e.
func GuardFree(e effect.Effect) Guard { return underGuard{e: e, not: true} }

// GuardDuring holds while the world's own entity is under e.
func GuardDuring(e effect.Effect) Guard { return duringGuard{e: e} }

// GuardWhile holds while the entity of is under e.
func GuardWhile(of func() uid.UID64, e effect.Effect) Guard { return duringGuard{e: e, of: of} }

// GuardPlaying holds while the entity, or the place Around turned to, plays the role of bit.
func GuardPlaying(bit uint8) Guard { return playingGuard{bit: bit} }

type underGuard struct {
	e   effect.Effect
	not bool
}

func (g underGuard) holds(c *ctx) bool { return c.entity && c.effects().Has(c.id, g.e) != g.not }

func (g underGuard) sign() string {
	if g.not {
		return fmt.Sprintf("free(%d)", g.e.Mark())
	}
	return fmt.Sprintf("under(%d)", g.e.Mark())
}

type duringGuard struct {
	e  effect.Effect
	of func() uid.UID64
}

func (g duringGuard) holds(c *ctx) bool {
	switch {
	case g.of != nil:
		return g.e.On(g.of())
	case c.instant:
		return g.e.On(c.pass.World)
	case c.sys.world != nil:
		return c.sys.effects.Has(c.sys.world(), g.e)
	}
	return false
}

func (g duringGuard) sign() string {
	if g.of != nil {
		return fmt.Sprintf("while(%d)", g.e.Mark())
	}
	return fmt.Sprintf("during(%d)", g.e.Mark())
}

type playingGuard struct{ bit uint8 }

func (g playingGuard) holds(c *ctx) bool { return c.entity && c.rolesOf(c.id)&(1<<g.bit) != 0 }
func (g playingGuard) sign() string      { return fmt.Sprintf("playing(%d)", g.bit) }

// NewBranch is a branch named name running node while every pred holds of P and every guard
// holds; through, in a plan, it goes on once begun until node is done.
func NewBranch[P any](name string, preds []func(P) bool, guards []Guard, through bool,
	node Step) Step {
	var b strings.Builder
	fmt.Fprintf(&b, "branch[%s]", typeName[P]())
	for _, p := range preds {
		b.WriteString(funcName(p))
	}
	for _, g := range guards {
		b.WriteString(g.sign())
	}
	if through {
		b.WriteString("through")
	}
	return composite{name: name, kids: []Step{node}, sign: b.String(),
		make: func() exec { return &branch[P]{preds: preds, guards: guards, through: through} }}
}

type branch[P any] struct {
	basic
	preds   []func(P) bool
	guards  []Guard
	through bool
	fact    *fact[P]
}

func (*branch[P]) instant() {}

func (*branch[P]) onFact() bool { return reflect.TypeFor[P]() != reflect.TypeFor[any]() }

func (b *branch[P]) refuses(payload any) string {
	if b.through {
		return "goes on past its moment, which a rule cannot"
	}
	if _, ok := payload.(*P); !ok && b.onFact() {
		return "is a condition on another moment"
	}
	return ""
}

func (b *branch[P]) bind(s *system) {
	if b.onFact() {
		b.fact = factOf[P](s)
	}
}

func (b *branch[P]) holds(v P) bool {
	for _, p := range b.preds {
		if !p(v) {
			return false
		}
	}
	return true
}

func (b *branch[P]) guarded(c *ctx) bool {
	for _, g := range b.guards {
		if !g.holds(c) {
			return false
		}
	}
	return true
}

func (b *branch[P]) tick(c *ctx, at int, kids []int) Status {
	if !b.guarded(c) {
		return Failure
	}
	if !b.onFact() {
		return c.run(kids[0])
	}
	if c.instant {
		if v, ok := c.payload.(*P); ok && b.holds(*v) {
			return c.run(kids[0])
		}
		return Failure
	}
	f := b.fact.of(c)
	if f == nil || !b.holds(*f) {
		if !b.through || !c.wasRunning(at) {
			return Failure
		}
		if f == nil {
			return c.run(kids[0])
		}
	}
	return c.under(*f, kids[0])
}

// NewEither runs a or b, as likely one as the other, drawn afresh at every step of the game.
func NewEither(a, b Step) Step { return NewFirst("", NewChance(0.5, a), b) }

// NewOnce casts e and runs node while the entity is not under e, and fails while it is.
func NewOnce(e effect.Effect, node Step) Step {
	return NewBranch[any]("", nil, []Guard{GuardFree(e)}, false, NewThen("", NewApply(e), node))
}
