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
	holds(c *ctx, at int) bool
	sign() string
}

// GuardUnder holds while the entity is under e.
func GuardUnder(e effect.Effect) Guard { return underGuard{e: e} }

// GuardNot holds while g does not.
func GuardNot(g Guard) Guard { return notGuard{g} }

// GuardDuring holds while the world's own entity is under e.
func GuardDuring(e effect.Effect) Guard { return duringGuard{e: e} }

// GuardWhile holds while the entity of is under e.
func GuardWhile(of func() uid.UID64, e effect.Effect) Guard { return duringGuard{e: e, of: of} }

// GuardPlaying holds while the entity, or the place Around turned to, plays the role of bit.
func GuardPlaying(bit uint8) Guard { return playingGuard{bit: bit} }

// GuardChance holds with the probability p, drawn afresh every step from the world's seed, the
// time and the entity, so a load and a replay draw alike.
func GuardChance(p float64) Guard { return chanceGuard{p: p} }

type underGuard struct{ e effect.Effect }

func (g underGuard) holds(c *ctx, _ int) bool { return c.entity && c.effects().Has(c.id, g.e) }
func (g underGuard) sign() string             { return fmt.Sprintf("under(%d)", g.e.Mark()) }

type notGuard struct{ g Guard }

func (g notGuard) holds(c *ctx, at int) bool { return !g.g.holds(c, at) }
func (g notGuard) sign() string              { return "not" + g.g.sign() }

type duringGuard struct {
	e  effect.Effect
	of func() uid.UID64
}

func (g duringGuard) holds(c *ctx, _ int) bool {
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

func (g playingGuard) holds(c *ctx, _ int) bool {
	return c.entity && c.rolesOf(c.id)&(1<<g.bit) != 0
}

func (g playingGuard) sign() string { return fmt.Sprintf("playing(%d)", g.bit) }

type chanceGuard struct{ p float64 }

func (g chanceGuard) holds(c *ctx, at int) bool {
	if !c.entity {
		return false
	}
	t, seed := c.now, uint64(0)
	if c.instant {
		t, seed = c.pass.Time, c.pass.Seed
	} else if c.sys != nil {
		seed = c.sys.seed
	}
	return draw(seed, t, uint64(c.id), hash(c.tree.name)+uint64(at)) < g.p
}

func (g chanceGuard) sign() string { return fmt.Sprintf("chance(%g)", g.p) }

// NewBranch is a branch named name running node when every pred holds of P and every guard
// holds. In a plan a branch of a fact runs once as the fact comes to hold, node to its end; one
// of any runs while its guards hold.
func NewBranch[P any](name string, preds []func(P) bool, guards []Guard, node Step) Step {
	var b strings.Builder
	fmt.Fprintf(&b, "branch[%s]", typeName[P]())
	for _, p := range preds {
		b.WriteString(funcName(p))
	}
	for _, g := range guards {
		b.WriteString(g.sign())
	}
	return composite{name: name, kids: []Step{node}, sign: b.String(),
		make: func() exec { return &branch[P]{preds: preds, guards: guards} }}
}

type branch[P any] struct {
	preds  []func(P) bool
	guards []Guard
	fact   *fact[P]
}

// what a branch of a fact keeps in its slot
const (
	armed uint8 = iota // the fact's coming, or one carried already, starts the branch
	spent              // it ran on the fact carried now: the fact must come afresh
	going              // node runs to its end, whatever the fact does
)

func (*branch[P]) instant() {}

func (*branch[P]) onFact() bool { return reflect.TypeFor[P]() != reflect.TypeFor[any]() }

func (b *branch[P]) refuses(payload any) string {
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

func (b *branch[P]) guarded(c *ctx, at int) bool {
	for _, g := range b.guards {
		if !g.holds(c, at) {
			return false
		}
	}
	return true
}

func (b *branch[P]) enter(*ctx, int) {}

func (b *branch[P]) halt(c *ctx, at int) {
	if b.onFact() && !c.instant {
		c.mind.Slot[at] = spent
	}
}

func (b *branch[P]) tick(c *ctx, at int, kids []int) Status {
	if !b.onFact() {
		if !b.guarded(c, at) {
			return Failure
		}
		return c.run(kids[0])
	}
	if c.instant {
		if v, ok := c.payload.(*P); ok && b.holds(*v) && b.guarded(c, at) {
			return c.run(kids[0])
		}
		return Failure
	}
	if c.mind.Slot[at] == going {
		return b.finish(c, at, kids)
	}
	f := b.fact.of(c)
	if f == nil || !b.holds(*f) || !b.guarded(c, at) {
		c.mind.Slot[at] = armed
		return Failure
	}
	if c.mind.Slot[at] == spent {
		return Failure
	}
	c.mind.Slot[at] = going
	return b.finish(c, at, kids)
}

func (b *branch[P]) finish(c *ctx, at int, kids []int) Status {
	var st Status
	if f := b.fact.of(c); f != nil {
		st = c.under(*f, kids[0])
	} else {
		st = c.run(kids[0])
	}
	if st != Running {
		c.mind.Slot[at] = spent
	}
	return st
}
