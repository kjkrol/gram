package topography

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Haze has the air haze the ground and what stands on it, through any camera, as thick as the
// weather makes it, or clear (Settings.Haze). Ctrl+F by default, in turn.
type Haze struct{ On bool }

func (Haze) Command() {}

var _ command.Command = Haze{}

// Care has the ground within Rings sectors of the middle of the screen shaded with full care and
// one care less every Rings farther ([Plugin.SetCare], Settings.Care), 0 every one. Alt+Shift+=
// and Alt+Shift+- by default, a sector, two, three, then every one, and back.
type Care struct{ Rings int }

func (Care) Command() {}

var _ command.Command = Care{}

// AdaptCare has the full care's reach follow the frame budget, or not ([Plugin.AdaptCare],
// Settings.AdaptiveCare): the cameras' frame budget (cameras.Config.Levers) steps it down the
// care's ladder before the pictures' resolution as the rate falls under, and back up after it
// as the rate stays over. The settings screen alone, no key.
type AdaptCare struct{ On bool }

func (AdaptCare) Command() {}

var _ command.Command = AdaptCare{}

// ShowCare has the care's bands drawn over the ground, or not ([Plugin.ShowCare],
// Settings.ShowCare): the boundaries between sectors of different care, the full care's green,
// then yellow-green, yellow and orange. The settings screen alone, no key.
type ShowCare struct{ Shown bool }

func (ShowCare) Command() {}

var _ command.Command = ShowCare{}

// Settings is what the topography lets the players adjust, saved with the game.
type Settings struct {
	Haze         bool
	Care         int
	AdaptiveCare bool
	ShowCare     bool
}

// Resources is what the topography publishes: its Settings.
type Resources struct {
	Settings Settings
}

// Queues are where the shaping's, the haze's and the care's commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return append(p.shaper.queues(), &p.hazing, &p.caring, &p.adapting, &p.showing)
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

// careMost is the top of the care's slider, every sector
const careMost = 8

// Settings are the haze and, on a square grid, whether the full care's reach follows the frame
// budget, how far out the ground is shaded with full care (a sector to careMost, the top every
// one) and whether the care's bands are shown (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	out := []control.Setting{control.Switch{
		Label:   "Haze over the ground",
		Section: control.Graphics,
		Get:     p.Hazes,
		Set:     func(on bool) command.Command { return Haze{On: on} },
	}}
	if p.ground == nil {
		return out
	}
	return append(out,
		control.Switch{
			Label:   "Full care follows the frame budget",
			Section: control.PictureCare,
			Get:     p.CareAdaptive,
			Set:     func(on bool) command.Command { return AdaptCare{On: on} },
		},
		control.Range{
			Label:   "Full care out to",
			Section: control.PictureCare,
			Min:     1,
			Max:     careMost,
			Step:    1,
			Get: func() float64 {
				if care := p.Care(); care > 0 {
					return float64(min(care, careMost))
				}
				return careMost
			},
			Set: func(v float64) command.Command {
				if v >= careMost {
					return Care{Rings: 0}
				}
				return Care{Rings: int(v)}
			},
			Shows: func(v float64) string {
				switch {
				case v >= careMost:
					return "every sector"
				case v <= 1:
					return "a sector"
				}
				return fmt.Sprintf("%d sectors", int(v))
			},
		},
		control.Switch{
			Label:   "Show the care",
			Section: control.PictureAreas,
			Get:     p.CareShown,
			Set:     func(on bool) command.Command { return ShowCare{Shown: on} },
		})
}
