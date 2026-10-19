package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Haze has the air haze the ground and what stands on it, through any camera, as thick as the
// weather makes it, or clear (Settings.Haze). Ctrl+F by default, in turn.
type Haze struct{ On bool }

func (Haze) Command() {}

var _ command.Command = Haze{}

// Care has the ground within Rings sectors of the middle of the screen shaded with full care
// ([Plugin.SetCare], Settings.Care), 0 every one. Alt+Shift+= and Alt+Shift+- by default, a
// sector, two, three, then every one, and back.
type Care struct{ Rings int }

func (Care) Command() {}

var _ command.Command = Care{}

// Settings is what the topography lets the players adjust, saved with the game.
type Settings struct {
	Haze bool
	Care int
}

// Resources is what the topography publishes: its Settings.
type Resources struct {
	Settings Settings
}

// Queues are where the shaping's, the haze's and the care's commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return append(p.shaper.queues(), &p.hazing, &p.caring)
}

// DefaultBindings are Shift+= and Shift+- raising and lowering the ground under the cursor (= and
// - alone zoom the camera), a left drag with L held levelling it, Ctrl+F the haze, Alt+Shift+=
// and Alt+Shift+- the full care farther and nearer.
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
			func(control.Context) (Haze, bool) { return Haze{On: !p.Hazes()}, true }),
		control.Build(
			control.KeyPress{Key: control.KeyEqual, Mods: control.Mods{Alt: true, Shift: true}},
			"Full care farther",
			func(control.Context) (Care, bool) { return Care{Rings: p.careStepped(1)}, true }),
		control.Build(
			control.KeyPress{Key: control.KeyMinus, Mods: control.Mods{Alt: true, Shift: true}},
			"Full care nearer",
			func(control.Context) (Care, bool) { return Care{Rings: p.careStepped(-1)}, true }),
	)
}

// Settings are the haze and, on a square grid, how far out the ground is shaded with full care
// (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	out := []control.Setting{control.Switch{
		Label: "Haze over the ground",
		Get:   p.Hazes,
		Set:   func(on bool) command.Command { return Haze{On: on} },
	}}
	if p.ground == nil {
		return out
	}
	return append(out, control.Choice{
		Label:   "Full care out to",
		Options: []string{"a sector", "two sectors", "three sectors", "every sector"},
		Get:     func() int { return careRung(p.Care()) },
		Set:     func(i int) command.Command { return Care{Rings: careLadder[i]} },
	})
}
