package main

import (
	"log"
	"os"
	"strings"

	"github.com/kjkrol/gram/control"
)

// PROBE_KEYS: keys pressed from the second second on, comma separated, one a tick, with the
// modifiers "Shift+", "Alt+" and "Ctrl+", e.g. "Shift+M,Alt+Minus,Alt+Shift+Minus"
var keys = map[string]control.Key{
	"F11": control.KeyF11, "H": control.KeyH, "M": control.KeyM, "G": control.KeyG,
	"Tab": control.KeyTab, "W": control.KeyW, "P": control.KeyP, "V": control.KeyV,
	"Equal": control.KeyEqual, "Minus": control.KeyMinus, "L": control.KeyL,
	"A": control.KeyA,
}

func (p *probe) play() {
	list := strings.Split(os.Getenv("PROBE_KEYS"), ",")
	i := p.tick - 120
	if i < 0 || i >= len(list) || list[i] == "" {
		return
	}
	name := list[i]
	var shift, alt, ctrl bool
	for {
		switch {
		case strings.HasPrefix(name, "Shift+"):
			shift, name = true, strings.TrimPrefix(name, "Shift+")
		case strings.HasPrefix(name, "Alt+"):
			alt, name = true, strings.TrimPrefix(name, "Alt+")
		case strings.HasPrefix(name, "Ctrl+"):
			ctrl, name = true, strings.TrimPrefix(name, "Ctrl+")
		default:
			goto parsed
		}
	}
parsed:
	key, ok := keys[name]
	if !ok {
		log.Printf("P:t%d unknown key %q", p.tick, name)
		return
	}
	ev := &control.InputEvents{}
	ev.Modifiers.Shift, ev.Modifiers.Alt, ev.Modifiers.Ctrl = shift, alt, ctrl
	if shift {
		ev.AddKeyEvent(control.KeyShift, control.ActionPress)
	}
	ev.AddKeyEvent(key, control.ActionPress)
	ev.AddKeyEvent(key, control.ActionRelease)
	if shift {
		ev.AddKeyEvent(control.KeyShift, control.ActionRelease)
	}
	p.a.players.EventHandler().HandleEvents(ev)
	log.Printf("P:t%d key %s", p.tick, list[i])
}
