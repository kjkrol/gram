package main

import (
	"log"

	"github.com/kjkrol/gram/control"
)

// Shift+L at the fifth second: the lamps' shadows
func (p *probe) play() {
	ev := &control.InputEvents{}
	switch p.tick {
	case 300:
		ev.Modifiers.Shift = true
		ev.AddKeyEvent(control.KeyShift, control.ActionPress)
		ev.AddKeyEvent(control.KeyL, control.ActionPress)
	case 302:
		ev.Modifiers.Shift = true
		ev.AddKeyEvent(control.KeyL, control.ActionRelease)
		ev.AddKeyEvent(control.KeyShift, control.ActionRelease)
	default:
		return
	}
	p.a.players.EventHandler().HandleEvents(ev)
	log.Printf("P:t%d Shift+L", p.tick)
}
