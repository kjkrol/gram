package steps

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// Guard is a condition a branch asks of somebody's state besides its fact or moment.
type Guard interface {
	holds(c *ctx, at int) bool
	sign() string
}

// Whose is whom a guard asks about.
type Whose struct {
	kind whoseKind
	of   func() uid.UID64
}

type whoseKind uint8

const (
	ofMe whoseKind = iota
	ofSubject
	ofTarget
	ofEntity
)

var (
	// OfMe is the entity the rule or the plan is about.
	OfMe = Whose{kind: ofMe}
	// OfSubject is the other of the pair the moment met, else the subject of the moment or
	// the fact; nobody while there is none.
	OfSubject = Whose{kind: ofSubject}
	// OfTarget is whom the steps act on now: the place Around turned to.
	OfTarget = Whose{kind: ofTarget}
)

// OfEntity is the entity of gives, a plugin's own.
func OfEntity(of func() uid.UID64) Whose { return Whose{kind: ofEntity, of: of} }

func (w Whose) resolve(c *ctx) (uid.UID64, bool) {
	switch w.kind {
	case ofMe:
		return c.me, c.entity
	case ofSubject:
		if c.hasOther {
			return c.other, true
		}
		if c.instant {
			if m, ok := c.payload.(met); ok {
				var other uid.UID64
				found := false
				m.Whom(func(id uid.UID64) {
					if !found {
						other, found = id, true
					}
				})
				return other, found
			}
			if s, ok := c.payload.(subject); ok {
				return s.Subject()
			}
			return 0, false
		}
		return c.subject, c.about
	case ofTarget:
		return c.id, c.entity
	}
	return w.of(), true
}

func (w Whose) sign() string {
	return [...]string{"me", "subject", "target", "entity"}[w.kind]
}

// Lowered is a condition laid down into a guard, by package cond.
type Lowered interface {
	Lower(as LowerAs, name string) Lowering
}

// LowerAs is where a condition stands: at the top of a rule, where the roles it names narrow
// the host; inside a branch or a plan, where they are guards; or as a loop's Until, where a
// fact that comes is allowed.
type LowerAs uint8

const (
	AtTop LowerAs = iota
	Inside
	AsUntil
)

// Lowering is a condition as a branch takes it: its guard, the roles it asks at the top of a
// rule (bits of the roles' family), the places it turns to, the moments its words are of and
// how many facts it waits for (MaxCame at most).
type Lowering struct {
	Guard     Guard // nil for always
	SelfRoles []uint8
	OtherRole int // -1 for none
	Rings     int
	Places    int
	Words     []reflect.Type
	Came      int
}

// MaxCame is how many facts one Until may wait for: a bit each in its slot.
const MaxCame = 8

// GuardUnder holds while w is under e.
func GuardUnder(e effect.Effect, w Whose) Guard { return underGuard{e: e, w: w} }

// GuardPlaying holds while w plays the role of bit.
func GuardPlaying(bit uint8, w Whose) Guard { return playingGuard{bit: bit, w: w} }

// GuardChance holds with the probability p, drawn afresh every step from the world's seed, the
// time and w, so a load and a replay draw alike.
func GuardChance(p float64, w Whose) Guard { return chanceGuard{p: p, w: w} }

// GuardAlways holds while w is somebody.
func GuardAlways(w Whose) Guard { return alwaysGuard{w: w} }

// GuardWord holds while holds does of the moment P a rule stands on, or of the fact P the
// entity carries in a plan, w being somebody.
func GuardWord[P any](name string, holds func(P) bool, w Whose) Guard {
	return &wordGuard[P]{name: name, of: holds, w: w}
}

// GuardCame holds once the fact F has come to the entity afresh since the loop began: one
// carried from before does not count. k is its bit in the Until's slot.
func GuardCame[F any](k uint8, w Whose) Guard { return &cameGuard[F]{k: k, w: w} }

// GuardNot holds while g does not.
func GuardNot(g Guard) Guard { return notGuard{g} }

// GuardAll holds while every guard does.
func GuardAll(gs ...Guard) Guard { return allGuard{gs} }

// GuardAny holds while any guard does.
func GuardAny(gs ...Guard) Guard { return anyGuard{gs} }

type underGuard struct {
	e effect.Effect
	w Whose
}

func (g underGuard) holds(c *ctx, _ int) bool {
	id, ok := g.w.resolve(c)
	return ok && c.effects().Has(id, g.e)
}

func (g underGuard) sign() string { return fmt.Sprintf("under(%d,%s)", g.e.Mark(), g.w.sign()) }

type playingGuard struct {
	bit uint8
	w   Whose
}

func (g playingGuard) holds(c *ctx, _ int) bool {
	id, ok := g.w.resolve(c)
	return ok && c.rolesOf(id)&(1<<g.bit) != 0
}

func (g playingGuard) sign() string { return fmt.Sprintf("playing(%d,%s)", g.bit, g.w.sign()) }

type chanceGuard struct {
	p float64
	w Whose
}

func (g chanceGuard) holds(c *ctx, at int) bool {
	id, ok := g.w.resolve(c)
	if !ok {
		return false
	}
	t, seed := c.now, uint64(0)
	if c.instant {
		t, seed = c.pass.Time, c.pass.Seed
	} else if c.sys != nil {
		seed = c.sys.seed
	}
	return draw(seed, t, uint64(id), hash(c.tree.name)+uint64(at)) < g.p
}

func (g chanceGuard) sign() string { return fmt.Sprintf("chance(%g,%s)", g.p, g.w.sign()) }

type alwaysGuard struct{ w Whose }

func (g alwaysGuard) holds(c *ctx, _ int) bool {
	_, ok := g.w.resolve(c)
	return ok
}

func (g alwaysGuard) sign() string { return "always(" + g.w.sign() + ")" }

type wordGuard[P any] struct {
	name string
	of   func(P) bool
	w    Whose
	fact *fact[P]
}

func (g *wordGuard[P]) bind(s *system) {
	if reflect.TypeFor[P]() != reflect.TypeFor[any]() {
		g.fact = factOf[P](s)
	}
}

func (g *wordGuard[P]) holds(c *ctx, _ int) bool {
	if _, ok := g.w.resolve(c); !ok {
		return false
	}
	if c.instant {
		v, ok := c.payload.(*P)
		return ok && g.of(*v)
	}
	if g.fact == nil {
		return false
	}
	f := g.fact.of(c)
	return f != nil && g.of(*f)
}

func (g *wordGuard[P]) sign() string { return fmt.Sprintf("word(%s,%s)", g.name, g.w.sign()) }

type cameGuard[F any] struct {
	k    uint8
	w    Whose
	fact *fact[F]
}

func (g *cameGuard[F]) bind(s *system) { g.fact = factOf[F](s) }

// the bit says the fact was absent since the loop began: present now, it came
func (g *cameGuard[F]) holds(c *ctx, at int) bool {
	if _, ok := g.w.resolve(c); !ok || g.fact == nil {
		return false
	}
	if g.fact.of(c) == nil {
		c.mind.Slot[at] |= 1 << g.k
		return false
	}
	return c.mind.Slot[at]&(1<<g.k) != 0
}

func (g *cameGuard[F]) sign() string {
	return fmt.Sprintf("came[%s](%d,%s)", typeName[F](), g.k, g.w.sign())
}

func bindGuard(g Guard, s *system) {
	if b, ok := g.(binder); ok {
		b.bind(s)
	}
}

type notGuard struct{ g Guard }

func (g notGuard) holds(c *ctx, at int) bool { return !g.g.holds(c, at) }
func (g notGuard) sign() string              { return "not" + g.g.sign() }
func (g notGuard) bind(s *system)            { bindGuard(g.g, s) }

type allGuard struct{ gs []Guard }

func (g allGuard) bind(s *system) {
	for _, k := range g.gs {
		bindGuard(k, s)
	}
}

func (g allGuard) holds(c *ctx, at int) bool {
	for _, k := range g.gs {
		if !k.holds(c, at) {
			return false
		}
	}
	return true
}

func (g allGuard) sign() string { return "all(" + signs(g.gs) + ")" }

type anyGuard struct{ gs []Guard }

func (g anyGuard) bind(s *system) {
	for _, k := range g.gs {
		bindGuard(k, s)
	}
}

func (g anyGuard) holds(c *ctx, at int) bool {
	for _, k := range g.gs {
		if k.holds(c, at) {
			return true
		}
	}
	return false
}

func (g anyGuard) sign() string { return "any(" + signs(g.gs) + ")" }

func signs(gs []Guard) string {
	var b strings.Builder
	for _, g := range gs {
		b.WriteString(g.sign())
	}
	return b.String()
}

// NewBranch is a branch named name running node while guard holds, of the moment or the fact
// P. In a plan a branch of a fact runs once as the fact comes to hold, node to its end; one
// of any runs while its guard holds. With until, a plan's, the branch goes on after node is
// done and ends well once until holds, node halted.
func NewBranch[P any](name string, guard Guard, node Step, until Guard) Step {
	sign := fmt.Sprintf("branch[%s]", typeName[P]())
	if guard != nil {
		sign += guard.sign()
	}
	kids := []Step{node}
	if until != nil {
		kids = append(kids, leaf{sign: "until(" + until.sign() + ")",
			make: func() exec { return untilNode{g: until} }})
	}
	return composite{name: name, kids: kids, sign: sign,
		make: func() exec { return &branch[P]{guard: guard, until: until != nil} }}
}

type untilNode struct{ g Guard }

func (u untilNode) bind(s *system)       { bindGuard(u.g, s) }
func (u untilNode) enter(c *ctx, at int) { c.mind.Slot[at] = 0 }
func (untilNode) halt(*ctx, int)         {}

func (u untilNode) tick(c *ctx, at int, _ []int) Status {
	if u.g.holds(c, at) {
		return Success
	}
	return Running
}

type branch[P any] struct {
	guard Guard
	until bool
	fact  *fact[P]
}

// what a branch of a fact keeps in its slot
const (
	armed uint8 = iota // the fact's coming, or one carried already, starts the branch
	spent              // it ran on the fact carried now: the fact must come afresh
	going              // node runs to its end, whatever the fact does
)

// thenDone is the bit of a branch's slot saying its node is done while its until is waited
// for, so the node is not run again
const thenDone uint8 = 0x80

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
	if b.guard != nil {
		bindGuard(b.guard, s)
	}
}

func (b *branch[P]) guarded(c *ctx, at int) bool {
	return b.guard == nil || b.guard.holds(c, at)
}

func (b *branch[P]) enter(c *ctx, at int) {
	if !b.onFact() && !c.instant {
		c.mind.Slot[at] = 0
	}
}

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
		return b.loop(c, at, kids, nil)
	}
	if c.instant {
		if _, ok := c.payload.(*P); ok && b.guarded(c, at) {
			return c.run(kids[0])
		}
		return Failure
	}
	if c.mind.Slot[at]&^thenDone == going {
		return b.finish(c, at, kids)
	}
	f := b.fact.of(c)
	if f == nil || !b.guarded(c, at) {
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
	st := b.loop(c, at, kids, b.fact.of(c))
	if st != Running {
		c.mind.Slot[at] = spent
	}
	return st
}

// with an until the loop ends well as it holds, the steps halted, and waits once they are
// done; under is the fact the steps run under, nil for none
func (b *branch[P]) loop(c *ctx, at int, kids []int, under *P) Status {
	run := func(kid int) Status {
		if under != nil {
			return c.under(*under, kid)
		}
		return c.run(kid)
	}
	if !b.until {
		return run(kids[0])
	}
	if run(kids[1]) == Success {
		return Success
	}
	if c.mind.Slot[at]&thenDone != 0 {
		return Running
	}
	switch run(kids[0]) {
	case Failure:
		return Failure
	case Success:
		c.mind.Slot[at] |= thenDone
	}
	return Running
}
