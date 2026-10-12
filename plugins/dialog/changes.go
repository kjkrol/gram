package dialog

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/uid"
)

// Change is what an answer changes in the game, made by Alter or AlterListener.
type Change struct {
	c        changer
	listener bool
}

// Alter is the change fn makes to the speaker's component T, told whom it talks with; a speaker
// without T is left as it is.
func Alter[T any](fn func(v *T, listener uid.UID64)) Change {
	return Change{c: &alter[T]{fn: fn}}
}

// AlterListener is the change fn makes to the listener's component T, told who speaks.
func AlterListener[T any](fn func(v *T, speaker uid.UID64)) Change {
	return Change{c: &alter[T]{fn: fn}, listener: true}
}

// Condition is when an answer is offered, made by When.
type Condition struct{ c checker }

// When holds while fn says so of the speaker's component T and whom it talks with; for a speaker
// without T it does not.
func When[T any](fn func(v *T, listener uid.UID64) bool) Condition {
	return Condition{c: &when[T]{fn: fn}}
}

type changer interface {
	bind(si *goke.SysInit)
	apply(on, other uid.UID64)
}

type checker interface {
	bind(si *goke.SysInit)
	holds(on, other uid.UID64) bool
}

type alter[T any] struct {
	fn    func(v *T, other uid.UID64)
	query *goke.Query
	comp  goke.Comp[T]
}

func (a *alter[T]) bind(si *goke.SysInit) { a.query = si.NewQueryBuilder(&a.comp).Build() }

func (a *alter[T]) apply(on, other uid.UID64) {
	if a.query.Seek(on) {
		a.fn(a.comp.At(a.query.Cursor()), other)
	}
}

type when[T any] struct {
	fn    func(v *T, other uid.UID64) bool
	query *goke.Query
	comp  goke.Comp[T]
}

func (w *when[T]) bind(si *goke.SysInit) { w.query = si.NewQueryBuilder(&w.comp).Build() }

func (w *when[T]) holds(on, other uid.UID64) bool {
	return w.query.Seek(on) && w.fn(w.comp.At(w.query.Cursor()), other)
}

// Changes is the register of what answers change in the game, by name.
type Changes struct {
	p      *Plugin
	byName map[string]Change
}

// Define registers ch as name. A name defined twice, or a Change not made by Alter or
// AlterListener, panics.
func (c *Changes) Define(name string, ch Change) {
	c.p.must(fmt.Sprintf("dialog change %q defined", name))
	if ch.c == nil {
		panic(fmt.Sprintf("dialog: the change %q is made by neither Alter nor AlterListener", name))
	}
	if _, ok := c.byName[name]; ok {
		panic(fmt.Sprintf("dialog: the change %q is defined already", name))
	}
	if c.byName == nil {
		c.byName = map[string]Change{}
	}
	c.byName[name] = ch
}

// Conditions is the register of when answers are offered, by name.
type Conditions struct {
	p      *Plugin
	byName map[string]Condition
}

// Define registers cond as name. A name defined twice, one an effect bears, or a Condition not
// made by When panics.
func (c *Conditions) Define(name string, cond Condition) {
	c.p.must(fmt.Sprintf("dialog condition %q defined", name))
	if cond.c == nil {
		panic(fmt.Sprintf("dialog: the condition %q is not made by When", name))
	}
	if _, ok := c.byName[name]; ok {
		panic(fmt.Sprintf("dialog: the condition %q is defined already", name))
	}
	if _, ok := c.p.w.Effects().Lookup(name); ok {
		panic(fmt.Sprintf("dialog: the condition %q is named as an effect is: an answer's if would not tell them apart", name))
	}
	if c.byName == nil {
		c.byName = map[string]Condition{}
	}
	c.byName[name] = cond
}

// ConversationsPart is the dialog plugin's part of a Stage's definition, after its commands and
// before its kinds.
var ConversationsPart = section.Define("Conversations", section.Commands, section.Kinds)

// Conversations is the dialog plugin's section of a Stage, for stage.With: f defines the changes
// and conditions and loads the conversations.
func Conversations(f func(d *Plugin) error) section.Section { return section.Of(ConversationsPart, f) }

func (p *Plugin) bind(si *goke.SysInit) {
	for _, ch := range p.changes.byName {
		ch.c.bind(si)
	}
	for _, cond := range p.conditions.byName {
		cond.c.bind(si)
	}
}

func (ch Change) apply(speaker, listener uid.UID64) {
	if ch.listener {
		ch.c.apply(listener, speaker)
		return
	}
	ch.c.apply(speaker, listener)
}

func (p *Plugin) must(what string) {
	if err := p.w.InSection(what, ConversationsPart); err != nil {
		panic(fmt.Sprintf("dialog: %v", err))
	}
}
