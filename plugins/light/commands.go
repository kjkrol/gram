package light

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// CoarseShadows has the shadows baked coarse, half as fine a side, or in full detail
// (Settings.Coarse). H by default, in turn.
type CoarseShadows struct{ On bool }

// LampShadows has the Count lamps nearest a picture's camera cast shadows, 0 none
// (Settings.LampShadows). Shift+L by default, off or as many as the Config says.
type LampShadows struct{ Count int }

// AdaptShadows has the shadows follow the cameras' frame budget, or not
// (Settings.AdaptiveShadows): the light is a cameras.Lever (cameras.Config.Levers), the lamps'
// shadows dropped first and the shadows baked coarse next as the rate falls under the budget,
// back the other way as it stays over. The settings screen alone.
type AdaptShadows struct{ On bool }

func (AdaptShadows) Command() {}

var _ command.Command = AdaptShadows{}

// Settings is what the light lets the players adjust, saved with the game.
type Settings struct {
	Coarse          bool
	LampShadows     int
	AdaptiveShadows bool
}

// Resources is what the light publishes: its Settings.
type Resources struct {
	Settings Settings
}

func (CoarseShadows) Command() {}
func (LampShadows) Command()   {}

var (
	_ command.Command = CoarseShadows{}
	_ command.Command = LampShadows{}
)

// Queues are where the light's commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.coarses, &p.lampSwitches, &p.adapts}
}

// DefaultBindings are H switching the shadows' detail and Shift+L the lamps' shadows.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyH},
			"Coarser shadows, a quarter of the work, or fine again",
			func(control.Context) (CoarseShadows, bool) {
				return CoarseShadows{On: !p.Res.Settings.Coarse}, true
			}),
		control.Build(control.KeyPress{Key: control.KeyL, Mods: control.Mods{Shift: true}},
			"The lamps' shadows off, or on again",
			func(control.Context) (LampShadows, bool) {
				return LampShadows{Count: p.lampsNext()}, true
			}),
	}
}

// lampsNext is what Shift+L switches to: none, or as many as the Config says
func (p *Plugin) lampsNext() int {
	if p.Res.Settings.LampShadows > 0 {
		return 0
	}
	return p.cfg.LampShadows
}

// Settings are whether the shadows follow the frame budget, the shadows' detail and, with
// lamps casting shadows at all (Config.LampShadows), whether they do, under Picture/Shadows
// (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	out := []control.Setting{control.Switch{
		Label:   "Shadows follow the frame budget",
		Section: control.PictureShadows,
		Get:     p.ShadowsAdaptive,
		Set:     func(on bool) command.Command { return AdaptShadows{On: on} },
	}, control.Switch{
		Label:   "Coarse shadows",
		Section: control.PictureShadows,
		Get:     func() bool { return p.Res.Settings.Coarse },
		Set:     func(on bool) command.Command { return CoarseShadows{On: on} },
	}}
	if p.cfg.LampShadows > 0 {
		out = append(out, control.Switch{
			Label:   "Lamps' shadows",
			Section: control.PictureShadows,
			Get:     func() bool { return p.Res.Settings.LampShadows > 0 },
			Set: func(on bool) command.Command {
				if on {
					return LampShadows{Count: p.cfg.LampShadows}
				}
				return LampShadows{Count: 0}
			},
		})
	}
	return out
}
