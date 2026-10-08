package dialog

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// Begin is the command by which an entity begins to talk with whom it is about (plugin.Aimed: the
// one a rule's moment names, the one seen) — at the node At, or where its Script says. It is
// refused while either of them talks already.
type Begin struct {
	At    string
	with  uid.UID64
	aimed bool
}

// Aim tells the command whom to talk with.
func (b *Begin) Aim(who uid.UID64) { b.with, b.aimed = who, true }

// Choose is the answer Index of those offered now in Speaker's conversation: a button of the
// window pinned to the speaker gives it, the speaker filled in (ui.About). Only a player the
// listener obeys may choose.
type Choose struct {
	Index   int
	Speaker uid.UID64
}

// About is the answer for the conversation of the entity the window is pinned to.
func (c Choose) About(of uid.UID64) any {
	c.Speaker = of
	return c
}

// Mood is the command by which an entity changes what it makes of whom it is about (plugin.Aimed)
// by By: a rule's Order — struck, it likes the striker less.
type Mood struct {
	By    int8
	with  uid.UID64
	aimed bool
}

// Aim tells the command whom the mood is towards.
func (m *Mood) Aim(who uid.UID64) { m.with, m.aimed = who, true }

var (
	_ plugin.Aimed = (*Begin)(nil)
	_ plugin.Aimed = (*Mood)(nil)
	_ ui.About     = Choose{}
)

// queues are where the commands land, for the talk system to carry out.
type queues struct {
	begins  control.Queue[Begin]
	chooses control.Queue[Choose]
	moods   control.Queue[Mood]
}

// Queues are Begin's, Choose's and Mood's — for the players plugin.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.told.begins, &p.told.chooses, &p.told.moods}
}

// DefaultBindings is none: a conversation is begun by rules and answered by the window's buttons.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }
