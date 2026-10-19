package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Haze switches the air's haze over the ground and what stands on it, through any camera, as
// thick as the weather makes it.
type Haze struct{}

func (Haze) Command() {}

var _ command.Command = Haze{}

// Queues are where the shaping's and the haze's commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return append(p.shaper.queues(), &p.hazing)
}

// DefaultBindings are Shift+= and Shift+- raising and lowering the ground under the cursor (= and
// - alone zoom the camera), a left drag with L held levelling it and Ctrl+F the haze.
func (p *Plugin) DefaultBindings() []control.Binding {
	at := func(c control.Context) geom.Vec { return c.World(c.Cursor) }
	return append([]control.Binding{},
		control.Build(control.KeyPress{Key: control.KeyEqual, Mods: control.Mods{Shift: true}},
			"Raise the ground",
			func(c control.Context) (Raise, bool) { return Raise{At: at(c)}, true }),
		control.Build(control.KeyPress{Key: control.KeyMinus, Mods: control.Mods{Shift: true}},
			"Lower the ground",
			func(c control.Context) (Lower, bool) { return Lower{At: at(c)}, true }),
		control.Build(control.Drag{Button: control.MouseButtonLeft, Mods: control.Mods{}.Holding(control.KeyL)}, "Level the ground",
			func(c control.Context) (Level, bool) {
				return Level{From: c.World(c.Start), To: c.World(c.Cursor)}, true
			}),
		control.Build(control.KeyPress{Key: control.KeyF, Mods: control.Mods{Ctrl: true}},
			"The haze over the ground, or clear",
			func(control.Context) (Haze, bool) { return Haze{}, true }),
	)
}
