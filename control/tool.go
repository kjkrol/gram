package control

import "github.com/kjkrol/ceremony/command"

// Tool is what a player holds in hand, by name. While it holds one, the bindings With it take
// their triggers from the bindings with none.
type Tool string

// Take has the player who gives it hold Tool in hand, putting down whatever it held.
type Take struct{ Tool Tool }

// Drop has the player who gives it put down the tool it holds.
type Drop struct{}

func (Take) Command() {}
func (Drop) Command() {}

var (
	_ command.Command = Take{}
	_ command.Command = Drop{}
)
