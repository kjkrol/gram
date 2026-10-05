package world

import (
	"fmt"

	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/rule"
)

// Castings are the commands about effects of a Stage, by name: Define says one — put an effect
// on, take it off or switch it, for whom, set off by whom — and Named is the command, for a key
// (control.Give), a script or a rule's Order. Reached through Plugin.Castings; each Stage's world
// has its own. One with a By is given whenever the entity it names Triggers, and every name the
// commands say is checked as the game starts: one nobody bears, or one two bear, stops it there.
type Castings struct {
	w      *Plugin
	byName map[string]rule.Casting
}

// Define says the command called name. Call it where the Stage defines its commands; a name
// defined twice, or a command that names nobody (On), panics.
func (c *Castings) Define(name string, cmd rule.Casting) {
	c.w.must(fmt.Sprintf("command %q defined", name), section.Commands)
	if name == "" {
		panic("world: a command needs a name")
	}
	if _, ok := c.byName[name]; ok {
		panic(fmt.Sprintf("world: the command %q is defined already", name))
	}
	if err := c.w.module.castings.take(cmd); err != nil {
		panic(fmt.Sprintf("%v (the command %q)", err, name))
	}
	if c.byName == nil {
		c.byName = map[string]rule.Casting{}
	}
	c.byName[name] = cmd
}

// Named is the command defined as name; an unknown name panics.
func (c *Castings) Named(name string) rule.Casting {
	cmd, ok := c.byName[name]
	if !ok {
		panic(fmt.Sprintf("world: no command is defined as %q", name))
	}
	return cmd
}
