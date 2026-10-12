package steps

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

// Step is a part of a rule or a plan, made by package rule.
type Step interface {
	lay(t *tree) int
}

type exec interface {
	enter(c *ctx, at int) // when it runs after a tick it did not run in
	tick(c *ctx, at int, kids []int) Status
	halt(c *ctx, at int) // stopped while running: its branch lost to another
}

type basic struct{}

func (basic) enter(*ctx, int) {}
func (basic) halt(*ctx, int)  {}

type composite struct {
	name string
	kids []Step
	make func() exec
	sign string
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

type leaf struct {
	make func() exec
	sign string
}

func (n leaf) lay(t *tree) int { return t.add(n.make(), n.sign) }

// NewFirst is a OneOf named name.
func NewFirst(name string, nodes ...Step) Step {
	return composite{name: name, kids: nodes, sign: "first", make: func() exec { return first{} }}
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

// NewThen is a Steps named name.
func NewThen(name string, nodes ...Step) Step {
	return composite{name: name, kids: nodes, sign: "then", make: func() exec { return then{} }}
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

func NewWait(d time.Duration) Step {
	return leaf{sign: fmt.Sprintf("wait(%v)", d), make: func() exec { return wait{d: d} }}
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

func NewTimeout(d time.Duration, node Step) Step {
	return composite{kids: []Step{node}, sign: fmt.Sprintf("timeout(%v)", d), make: func() exec { return timeout{d: d} }}
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

func NewCooldown(d time.Duration, node Step) Step {
	return composite{kids: []Step{node}, sign: fmt.Sprintf("cooldown(%v)", d), make: func() exec { return cooldown{d: d} }}
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

func NewIdle() Step { return leaf{sign: "idle", make: func() exec { return idle{} }} }

type idle struct{ basic }

func (idle) tick(*ctx, int, []int) Status { return Running }

func typeName[T any]() string { return reflect.TypeFor[T]().String() }

// two conditions on one fact tell apart in the signature by it
func funcName(f any) string {
	if fn := runtime.FuncForPC(reflect.ValueOf(f).Pointer()); fn != nil {
		return fn.Name()
	}
	return "?"
}
