package light

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// CoarseShadows switches the shadows between their full detail and a coarser bake, half as fine a
// side. H by default.
type CoarseShadows struct{}

// LampShadows switches the lamps' shadows off, or on again as many as the Config says. Shift+L
// by default.
type LampShadows struct{}

func (CoarseShadows) Command() {}
func (LampShadows) Command()   {}

var (
	_ command.Command = CoarseShadows{}
	_ command.Command = LampShadows{}
)

// Queues are where the light's commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.coarses, &p.lampSwitches}
}

// DefaultBindings are H switching the shadows' detail and Shift+L the lamps' shadows.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyH},
			"Coarser shadows, a quarter of the work, or fine again",
			func(control.Context) (CoarseShadows, bool) { return CoarseShadows{}, true }),
		control.Build(control.KeyPress{Key: control.KeyL, Mods: control.Mods{Shift: true}},
			"The lamps' shadows off, or on again",
			func(control.Context) (LampShadows, bool) { return LampShadows{}, true }),
	}
}
