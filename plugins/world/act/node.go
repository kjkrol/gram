package act

import (
	"fmt"
	"reflect"
	"runtime"
	"time"
)

// Status is how a node stands after a tick.
type Status uint8

const (
	// Running is a node not done yet: it goes on at the next tick.
	Running Status = iota
	// Success is a node done well.
	Success
	// Failure is a node that could not.
	Failure
)

// Node is a part of a tree, made by a Branch's methods — or a plugin's, built on them.
type Node interface {
	// lay adds the node and those under it to t, returning its index.
	lay(t *tree) int
}

// Instant is a node done within the pass it runs in, made by a Reaction's methods: all a
// trigger's body takes, and a part of a tree as well.
type Instant interface {
	Node
	instant()
}

// quick is a node made instant, as a Reaction hands it out.
type quick struct{ Node }

func (quick) instant() {}

// bare is n without the wrapping that made it instant.
func bare(n Node) Node {
	if q, ok := n.(quick); ok {
		return bare(q.Node)
	}
	return n
}

// step is a node as a tree runs it.
type step interface {
	// enter readies the node when it runs after a tick it did not run in.
	enter(c *ctx, at int)
	// tick runs it for one tick.
	tick(c *ctx, at int, kids []int) Status
	// halt stops it while it runs: its branch lost to another.
	halt(c *ctx, at int)
}

// basic gives a step nothing to ready and nothing to stop.
type basic struct{}

func (basic) enter(*ctx, int) {}
func (basic) halt(*ctx, int)  {}

// composite is a node over others, laid out in order.
type composite struct {
	name string
	kids []Node
	make func() step
	sign string // what it is, for the tree's signature
}

func (n composite) lay(t *tree) int {
	at := t.add(n.make(), n.sign+"("+n.name+")")
	var kids []int
	for _, k := range n.kids {
		kids = append(kids, k.lay(t))
	}
	t.nodes[at].kids = kids
	return at
}

// leaf is a node over none.
type leaf struct {
	make func() step
	sign string
}

func (n leaf) lay(t *tree) int { return t.add(n.make(), n.sign) }

// newFirst is a First named name.
func newFirst(name string, nodes ...Node) Node {
	return composite{name: name, kids: nodes, sign: "first", make: func() step { return first{} }}
}

type first struct{ basic }

func (first) instant() {}

func (first) tick(c *ctx, _ int, kids []int) Status {
	for _, k := range kids {
		if s := c.run(k); s != Failure {
			return s
		}
	}
	return Failure
}

// newThen is a Then named name.
func newThen(name string, nodes ...Node) Node {
	return composite{name: name, kids: nodes, sign: "then", make: func() step { return then{} }}
}

type then struct{}

func (then) instant() {}

func (then) enter(c *ctx, at int) { c.mind.Slot[at] = 0 }
func (then) halt(*ctx, int)       {}

func (then) tick(c *ctx, at int, kids []int) Status {
	for k := int(c.mind.Slot[at]); k < len(kids); k++ {
		switch c.run(kids[k]) {
		case Running:
			c.mind.Slot[at] = uint8(k)
			return Running
		case Failure:
			return Failure
		}
	}
	return Success
}

// newWhen is a When named name over node.
func newWhen[F any](name string, node Node) Node {
	return composite{name: name, kids: []Node{node}, sign: "when[" + typeName[F]() + "]",
		make: func() step { return &when[F]{} }}
}

type when[F any] struct {
	basic
	fact *fact[F]
}

func (w *when[F]) bind(s *system) { w.fact = factOf[F](s) }

func (w *when[F]) tick(c *ctx, _ int, kids []int) Status {
	f := w.fact.of(c)
	if f == nil {
		return Failure
	}
	return c.under(*f, kids[0])
}

// newOn is an On named name over node.
func newOn[F any](name string, node Node) Node {
	return composite{name: name, kids: []Node{node}, sign: "on[" + typeName[F]() + "]",
		make: func() step { return &on[F]{} }}
}

type on[F any] struct {
	basic
	fact *fact[F]
}

func (o *on[F]) bind(s *system) { o.fact = factOf[F](s) }

func (o *on[F]) tick(c *ctx, at int, kids []int) Status {
	f := o.fact.of(c)
	if !c.wasRunning(at) && f == nil {
		return Failure
	}
	if f == nil {
		return c.run(kids[0])
	}
	return c.under(*f, kids[0])
}

// newIf is an If over node: in a tree F is a fact, in a trigger the moment it fires on.
func newIf[F any](holds func(F) bool, node Node) Node {
	return composite{kids: []Node{node}, sign: "if[" + typeName[F]() + "]" + funcName(holds),
		make: func() step { return &cond[F]{holds: holds} }}
}

type cond[F any] struct {
	basic
	holds func(F) bool
	fact  *fact[F]
}

func (*cond[F]) instant() {}

func (i *cond[F]) bind(s *system) { i.fact = factOf[F](s) }

func (i *cond[F]) tick(c *ctx, _ int, kids []int) Status {
	if c.instant { // a trigger's condition is on its moment
		if v, ok := c.payload.(*F); ok && i.holds(*v) {
			return c.run(kids[0])
		}
		return Failure
	}
	f := i.fact.of(c)
	if f == nil || !i.holds(*f) {
		return Failure
	}
	return c.under(*f, kids[0])
}

func newWait(d time.Duration) Node {
	return leaf{sign: fmt.Sprintf("wait(%v)", d), make: func() step { return wait{d: d} }}
}

type wait struct {
	basic
	d time.Duration
}

func (w wait) enter(c *ctx, at int) { c.mind.Since[at] = c.now }

func (w wait) tick(c *ctx, at int, _ []int) Status {
	if c.now-c.mind.Since[at] >= w.d {
		return Success
	}
	return Running
}

func newTimeout(d time.Duration, node Node) Node {
	return composite{kids: []Node{node}, sign: fmt.Sprintf("timeout(%v)", d), make: func() step { return timeout{d: d} }}
}

type timeout struct {
	basic
	d time.Duration
}

func (t timeout) enter(c *ctx, at int) { c.mind.Since[at] = c.now }

func (t timeout) tick(c *ctx, at int, kids []int) Status {
	if c.now-c.mind.Since[at] >= t.d {
		return Failure
	}
	return c.run(kids[0])
}

func newCooldown(d time.Duration, node Node) Node {
	return composite{kids: []Node{node}, sign: fmt.Sprintf("cooldown(%v)", d), make: func() step { return cooldown{d: d} }}
}

type cooldown struct {
	basic
	d time.Duration
}

func (cd cooldown) tick(c *ctx, at int, kids []int) Status {
	if c.now < c.mind.Since[at] {
		return Failure
	}
	s := c.run(kids[0])
	if s != Running {
		c.mind.Since[at] = c.now + cd.d
	}
	return s
}

func newInvert(node Node) Node {
	return composite{kids: []Node{node}, sign: "invert", make: func() step { return invert{} }}
}

type invert struct{ basic }

func (invert) instant() {}

func (invert) tick(c *ctx, _ int, kids []int) Status {
	switch s := c.run(kids[0]); s {
	case Success:
		return Failure
	case Failure:
		return Success
	default:
		return s
	}
}

func newIdle() Node { return leaf{sign: "idle", make: func() step { return idle{} }} }

type idle struct{ basic }

func (idle) tick(*ctx, int, []int) Status { return Running }

// typeName is T's name as a tree's signature writes it.
func typeName[T any]() string { return reflect.TypeFor[T]().String() }

// funcName is f's name, for a tree's signature: two conditions on one fact tell apart by it.
func funcName(f any) string {
	if fn := runtime.FuncForPC(reflect.ValueOf(f).Pointer()); fn != nil {
		return fn.Name()
	}
	return "?"
}
