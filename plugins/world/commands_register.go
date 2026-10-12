package world

import (
	"fmt"

	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/rule"
)

// Commands is a Stage's register of commands about effects, by name. As the first step begins a
// name a command says that nobody bears, or one two bear, panics.
type Commands struct {
	w      *Plugin
	byName map[string]rule.Command
}

// Define registers cmd as name. A name defined twice, or a command that names nobody, panics.
func (c *Commands) Define(name string, cmd rule.Command) {
	c.w.must(fmt.Sprintf("command %q defined", name), section.Commands)
	if name == "" {
		panic("world: a command needs a name")
	}
	if _, ok := c.byName[name]; ok {
		panic(fmt.Sprintf("world: the command %q is defined already", name))
	}
	cmd.Name = name
	if err := c.w.module.effectCmds.take(cmd); err != nil {
		panic(fmt.Sprintf("%v (the command %q)", err, name))
	}
	if c.byName == nil {
		c.byName = map[string]rule.Command{}
	}
	c.byName[name] = cmd
}

// Named is the command defined as name; an unknown name panics.
func (c *Commands) Named(name string) rule.Command {
	cmd, ok := c.Lookup(name)
	if !ok {
		panic(fmt.Sprintf("world: no command is defined as %q", name))
	}
	return cmd
}

// Lookup is the command defined as name, false for an unknown one.
func (c *Commands) Lookup(name string) (rule.Command, bool) {
	cmd, ok := c.byName[name]
	return cmd, ok
}
