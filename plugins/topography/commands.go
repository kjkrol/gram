package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Haze has the air haze the ground and what stands on it, through any camera, as thick as the
// weather makes it, or clear (Settings.Haze). The settings screen alone.
type Haze struct{ On bool }

func (Haze) Command() {}

var _ command.Command = Haze{}

// Settings is what the topography lets the players adjust, saved with the game.
type Settings struct {
	Haze bool
}

// Resources is what the topography publishes: its Settings.
type Resources struct {
	Settings Settings
}

// Queues are where the shaping's and the haze's commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return append(p.shaper.queues(), &p.hazing)
}

// DefaultBindings are Shift+= and Shift+- raising and lowering the ground under the cursor (= and
// - alone zoom the camera) and a left drag with L held levelling it.
func (p *Plugin) DefaultBindings() []control.Binding {
	at := func(c control.Context) geom.Vec { return c.World(c.Cursor) }
	return []control.Binding{
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
	}
}

// Settings are the haze (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	return []control.Setting{control.Switch{
		Label:   "Haze over the ground",
		Section: control.Graphics,
		Get:     p.read.Hazes,
		Set:     func(on bool) command.Command { return Haze{On: on} },
	}}
}
