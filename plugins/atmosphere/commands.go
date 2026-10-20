package atmosphere

import (
	"fmt"
	"slices"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
)

// Freeze stops the light of the day at the hour it stands (Settings.Frozen), or lets a frozen
// light go again. Only the light stops; the calendar and the weather go on. The settings screen
// alone.
type Freeze struct{ On bool }

// Hour puts a frozen light At a part of the day, 0 to 1 round midnight (Settings.Hour). The
// settings screen alone.
type Hour struct{ At float32 }

// ChangeWeather has the weather go on to its next state now.
type ChangeWeather struct{}

// SetWeather has the weather go into the state called Name now (weather.Clear, weather.Storm, a
// game's own); a name the climate lacks changes nothing.
type SetWeather struct{ Name string }

// Clouds has the clouds drawn in the Look (CloudLook, Settings.Clouds). The settings screen
// alone.
type Clouds struct{ Look CloudLook }

// AdaptClouds has the clouds' look follow the cameras' frame budget, or not
// (Settings.AdaptiveClouds): the atmosphere is a cameras.Lever (cameras.Config.Levers), the
// look stepped down from all as bodies to the low as bodies, flat and their shadows alone as
// the rate falls under the budget, and back up as it stays over. The settings screen alone.
type AdaptClouds struct{ On bool }

func (AdaptClouds) Command() {}

var _ command.Command = AdaptClouds{}

// HighShade has the high clouds shade the ground too, as the game starts, or no longer: the low
// ones alone do (Settings.HighShade). The settings screen alone.
type HighShade struct{ On bool }

// Settings is what the atmosphere lets the players adjust, saved with the game: the frozen
// light and its hour, the clouds' look, the high clouds' shadows.
type Settings struct {
	Frozen         bool
	Hour           float32
	Clouds         CloudLook
	HighShade      bool
	AdaptiveClouds bool
}

// Resources is what the atmosphere publishes: its Settings.
type Resources struct {
	Settings Settings
}

// Downpour has it rain now: SetWeather of weather.Rain.
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
		&p.shades, &p.adapts, &p.downpours}
}

// DefaultBindings are none: the light, the clouds and the weather are the settings screen's.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }

// Settings are the frozen light and its hour, the clouds' look and the high clouds' shadows
// (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	looks := make([]string, ShadowsAlone+1)
	for l := range looks {
		looks[l] = CloudLook(l).String()
	}
	out := []control.Setting{
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
			Options: looks,
			Get:     func() int { return int(p.ShownClouds()) },
			Set:     func(i int) command.Command { return Clouds{Look: CloudLook(i)} },
		},
		control.Switch{
			Label: "Clouds follow the frame budget",
			Get:   p.CloudsAdaptive,
			Set:   func(on bool) command.Command { return AdaptClouds{On: on} },
		},
		control.Switch{
			Label: "High clouds' shadows",
			Get:   func() bool { return p.shadows.High },
			Set:   func(on bool) command.Command { return HighShade{On: on} },
		},
	}
	weathers := p.climate.Config().Weathers
	if len(weathers) == 0 {
		return out
	}
	names := make([]string, len(weathers))
	for i, w := range weathers {
		names[i] = w.Name
	}
	return append(out, control.Choice{
		Label:   "Weather",
		Options: names,
		Get:     func() int { return max(slices.Index(names, p.climate.Now()), 0) },
		Set:     func(i int) command.Command { return SetWeather{Name: names[i]} },
	})
}

func (p *Plugin) carryOut() {
	p.freezes.Drain(func(i command.Issued[Freeze]) { p.sky.SetFrozen(i.Command.On) })
	p.hours.Drain(func(i command.Issued[Hour]) { p.sky.SetHour(i.Command.At) })
	p.changes.Drain(func(command.Issued[ChangeWeather]) { p.climate.Change() })
	p.sets.Drain(func(i command.Issued[SetWeather]) { p.climate.Set(i.Command.Name) })
	p.adapts.Drain(func(i command.Issued[AdaptClouds]) { p.AdaptClouds(i.Command.On) })
	p.clouds.Drain(func(i command.Issued[Clouds]) { p.ShowClouds(i.Command.Look) })
	p.shades.Drain(func(i command.Issued[HighShade]) { p.shadeHigh(i.Command.On) })
	p.downpours.Drain(func(command.Issued[Downpour]) { p.climate.Set(weather.Rain) })
	p.Res.Settings.Frozen, p.Res.Settings.Hour = p.sky.Frozen(), p.sky.Hour()
}
