package atmosphere

import (
	"fmt"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
)

// Freeze stops the light of the day at the hour it stands (Settings.Frozen), or lets a frozen
// light go again. Only the light stops; the calendar and the weather go on. P by default, in
// turn.
type Freeze struct{ On bool }

// Hour puts a frozen light At a part of the day, 0 to 1 round midnight (Settings.Hour). Shift+]
// and Shift+[ by default, half an hour on and back, while the light is frozen.
type Hour struct{ At float32 }

// ChangeWeather has the weather go on to its next state now.
type ChangeWeather struct{}

// SetWeather has the weather go into the state called Name now (weather.Clear, weather.Storm, a
// game's own); a name the climate lacks changes nothing.
type SetWeather struct{ Name string }

// Clouds has the clouds drawn in the Look (CloudLook, Settings.Clouds). Shift+V by default, in
// turn: flat, the low ones as bodies, all, their shadows alone, flat again.
type Clouds struct{ Look CloudLook }

// HighShade has the high clouds shade the ground too, as the game starts, or no longer: the low
// ones alone do (Settings.HighShade). Shift+S by default, in turn.
type HighShade struct{ On bool }

// Settings is what the atmosphere lets the players adjust, saved with the game: the frozen
// light and its hour, the clouds' look, the high clouds' shadows.
type Settings struct {
	Frozen    bool
	Hour      float32
	Clouds    CloudLook
	HighShade bool
}

// Resources is what the atmosphere publishes: its Settings.
type Resources struct {
	Settings Settings
}

// Downpour has it rain now: SetWeather of weather.Rain, a key to look at the rain with.
type Downpour struct{}

func (Freeze) Command()        {}
func (Hour) Command()          {}
func (ChangeWeather) Command() {}
func (SetWeather) Command()    {}
func (Clouds) Command()        {}
func (HighShade) Command()     {}
func (Downpour) Command()      {}

var (
	_ command.Command = Freeze{}
	_ command.Command = Hour{}
	_ command.Command = ChangeWeather{}
	_ command.Command = SetWeather{}
	_ command.Command = Clouds{}
	_ command.Command = HighShade{}
	_ command.Command = Downpour{}
)

// Queues are where the atmosphere's commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.freezes, &p.hours, &p.changes, &p.sets, &p.clouds,
		&p.shades, &p.downpours}
}

// DefaultBindings are P (Freeze), Shift+] and Shift+[ (Hour, half an hour on and back),
// Shift+V (Clouds) and, with the camera free, Shift+W (ChangeWeather) and Shift+S (HighShade):
// riding, S with Shift sprints back; Shift+U (Downpour).
func (p *Plugin) DefaultBindings() []control.Binding {
	shift := control.Mods{Shift: true}
	shifted := func(by float32) func(control.Context) (Hour, bool) {
		return func(control.Context) (Hour, bool) {
			return Hour{At: p.sky.Hour() + by}, p.sky.Frozen()
		}
	}
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyP}, "Freeze the light of the day",
			func(control.Context) (Freeze, bool) { return Freeze{On: !p.sky.Frozen()}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketRight, Mods: shift},
			"Frozen light half an hour later", shifted(sky.HalfHour)),
		control.Build(control.KeyPress{Key: control.KeyBracketLeft, Mods: shift},
			"Frozen light half an hour earlier", shifted(-sky.HalfHour)),
		control.Build(control.KeyPress{Key: control.KeyW, Mods: shift}, "Change the weather",
			func(control.Context) (ChangeWeather, bool) { return ChangeWeather{}, true },
		).In(camera.Outside),
		control.Build(control.KeyPress{Key: control.KeyV, Mods: shift},
			"Clouds flat, the low as bodies, all, shadows alone",
			func(control.Context) (Clouds, bool) {
				return Clouds{Look: (p.ShownClouds() + 1) % (ShadowsAlone + 1)}, true
			}),
		control.Build(control.KeyPress{Key: control.KeyS, Mods: shift},
			"The high clouds' shadows, or the low ones' alone",
			func(control.Context) (HighShade, bool) { return HighShade{On: !p.shadows.High}, true },
		).In(camera.Outside),
		control.Build(control.KeyPress{Key: control.KeyU, Mods: shift}, "Rain now",
			func(control.Context) (Downpour, bool) { return Downpour{}, true }),
	}
}

// Settings are the frozen light and its hour, the clouds' look and the high clouds' shadows
// (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	looks := make([]string, ShadowsAlone+1)
	for l := range looks {
		looks[l] = CloudLook(l).String()
	}
	return []control.Setting{
		control.Switch{
			Label: "Frozen light",
			Get:   p.sky.Frozen,
			Set:   func(on bool) command.Command { return Freeze{On: on} },
		},
		control.Range{
			Label: "Hour of the frozen light",
			Min:   0,
			Max:   23.5,
			Step:  0.5,
			Get:   func() float64 { return float64(p.sky.Hour()) * 24 },
			Set:   func(v float64) command.Command { return Hour{At: float32(v / 24)} },
			Shows: func(v float64) string { return fmt.Sprintf("%02d:%02d", int(v), int(v*60)%60) },
		},
		control.Choice{
			Label:   "Clouds",
			Section: control.Graphics,
			Options: looks,
			Get:     func() int { return int(p.ShownClouds()) },
			Set:     func(i int) command.Command { return Clouds{Look: CloudLook(i)} },
		},
		control.Switch{
			Label:   "High clouds' shadows",
			Section: control.Graphics,
			Get:     func() bool { return p.shadows.High },
			Set:     func(on bool) command.Command { return HighShade{On: on} },
		},
	}
}

func (p *Plugin) carryOut() {
	p.freezes.Drain(func(i command.Issued[Freeze]) { p.sky.SetFrozen(i.Command.On) })
	p.hours.Drain(func(i command.Issued[Hour]) { p.sky.SetHour(i.Command.At) })
	p.changes.Drain(func(command.Issued[ChangeWeather]) { p.climate.Change() })
	p.sets.Drain(func(i command.Issued[SetWeather]) { p.climate.Set(i.Command.Name) })
	p.clouds.Drain(func(i command.Issued[Clouds]) { p.ShowClouds(i.Command.Look) })
	p.shades.Drain(func(i command.Issued[HighShade]) { p.shadeHigh(i.Command.On) })
	p.downpours.Drain(func(command.Issued[Downpour]) { p.climate.Set(weather.Rain) })
	p.Res.Settings.Frozen, p.Res.Settings.Hour = p.sky.Frozen(), p.sky.Hour()
}
