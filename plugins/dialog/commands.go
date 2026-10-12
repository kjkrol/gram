package dialog

import (
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/ui"
	"github.com/kjkrol/uid"
)

// Begin is the command by which an entity begins to talk with whom it is aimed at, at the node
// At or where its Script says. It is refused while either of them talks already.
type Begin struct {
	At    string
	with  uid.UID64
	aimed bool
}

// Aim tells the command whom to talk with.
func (b *Begin) Aim(who uid.UID64) { b.with, b.aimed = who, true }

// Choose is the answer Index of those offered now in Speaker's conversation. Only a player the
// listener obeys may choose.
type Choose struct {
	Index   int
	Speaker uid.UID64
}

// About is the answer for the conversation of the entity of.
func (c Choose) About(of uid.UID64) control.Command {
	c.Speaker = of
	return c
}

// Feel is the command by which an entity undergoes the game's change called Change towards whom
// it is aimed at, as if it spoke and that one answered.
type Feel struct {
	Change string
	with   uid.UID64
	aimed  bool
}

func (Begin) Command()  {}
func (Choose) Command() {}
func (Feel) Command()   {}

var (
	_ control.Command = Begin{}
	_ control.Command = Choose{}
	_ control.Command = Feel{}
)

// Aim tells the command whom the change is towards.
func (f *Feel) Aim(who uid.UID64) { f.with, f.aimed = who, true }

var (
	_ plugin.Aimed = (*Begin)(nil)
	_ plugin.Aimed = (*Feel)(nil)
	_ ui.About     = Choose{}
)

type queues struct {
	begins  control.Queue[Begin]
	chooses control.Queue[Choose]
	feels   control.Queue[Feel]
}

// Queues are the plugin's command queues: Begin, Choose and Feel.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.told.begins, &p.told.chooses, &p.told.feels}
}

// DefaultBindings is none: a conversation is begun by rules and answered by the window's buttons.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }
