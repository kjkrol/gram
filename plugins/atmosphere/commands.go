package atmosphere

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
)

// Freeze stops the light of the day at the hour it stands, or lets a frozen light go again. Only
// the light stops; the calendar and the weather go on.
type Freeze struct{}

// Later moves a frozen light half an hour on; it does nothing while the light goes with the
// calendar.
type Later struct{}

// Earlier moves a frozen light half an hour back.
type Earlier struct{}

// ChangeWeather has the weather go on to its next state now.
type ChangeWeather struct{}

// SetWeather has the weather go into the state called Name now (weather.Clear, weather.Storm, a
// game's own); a name the climate lacks changes nothing.
type SetWeather struct{ Name string }

// Clouds has the clouds drawn in the next look (CloudLook): flat, the low ones as bodies, all,
// their shadows alone, flat again.
type Clouds struct{}

// HighShade has the high clouds shade the ground too, as the game starts, or no longer: the low
// ones alone do.
type HighShade struct{}

// Downpour has it rain now: SetWeather of weather.Rain, a key to look at the rain with.
type Downpour struct{}

func (Freeze) Command()        {}
func (Later) Command()         {}
func (Earlier) Command()       {}
func (ChangeWeather) Command() {}
func (SetWeather) Command()    {}
func (Clouds) Command()        {}
func (HighShade) Command()     {}
func (Downpour) Command()      {}

var (
	_ command.Command = Freeze{}
	_ command.Command = Later{}
	_ command.Command = Earlier{}
	_ command.Command = ChangeWeather{}
	_ command.Command = SetWeather{}
	_ command.Command = Clouds{}
	_ command.Command = HighShade{}
	_ command.Command = Downpour{}
)

// Queues are where the atmosphere's commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.freezes, &p.laters, &p.earliers, &p.changes, &p.sets,
		&p.clouds, &p.shades, &p.downpours}
}

// DefaultBindings are P (Freeze), Shift+] and Shift+[ (Later, Earlier), Shift+V (Clouds) and,
// with the camera free, Shift+W (ChangeWeather) and Shift+S (HighShade): riding, S with Shift
// sprints back; Shift+U (Downpour).
func (p *Plugin) DefaultBindings() []control.Binding {
	shift := control.Mods{Shift: true}
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyP}, "Freeze the light of the day", func(control.Context) (Freeze, bool) { return Freeze{}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketRight, Mods: shift}, "Frozen light half an hour later", func(control.Context) (Later, bool) { return Later{}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketLeft, Mods: shift}, "Frozen light half an hour earlier", func(control.Context) (Earlier, bool) { return Earlier{}, true }),
		control.Build(control.KeyPress{Key: control.KeyW, Mods: shift}, "Change the weather", func(control.Context) (ChangeWeather, bool) { return ChangeWeather{}, true }).In(camera.Outside),
		control.Build(control.KeyPress{Key: control.KeyV, Mods: shift},
			"Clouds flat, the low as bodies, all, shadows alone",
			func(control.Context) (Clouds, bool) { return Clouds{}, true }),
		control.Build(control.KeyPress{Key: control.KeyS, Mods: shift},
			"The high clouds' shadows, or the low ones' alone",
			func(control.Context) (HighShade, bool) { return HighShade{}, true },
		).In(camera.Outside),
		control.Build(control.KeyPress{Key: control.KeyU, Mods: shift}, "Rain now",
			func(control.Context) (Downpour, bool) { return Downpour{}, true }),
	}
}

func (p *Plugin) carryOut() {
	p.freezes.Drain(func(command.Issued[Freeze]) { p.sky.SetFrozen(!p.sky.Frozen()) })
	p.laters.Drain(func(command.Issued[Later]) { p.sky.Shift(sky.HalfHour) })
	p.earliers.Drain(func(command.Issued[Earlier]) { p.sky.Shift(-sky.HalfHour) })
	p.changes.Drain(func(command.Issued[ChangeWeather]) { p.climate.Change() })
	p.sets.Drain(func(i command.Issued[SetWeather]) { p.climate.Set(i.Command.Name) })
	p.clouds.Drain(func(command.Issued[Clouds]) { p.ShowClouds((p.look + 1) % (ShadowsAlone + 1)) })
	p.shades.Drain(func(command.Issued[HighShade]) { p.shadows.High = !p.shadows.High })
	p.downpours.Drain(func(command.Issued[Downpour]) { p.climate.Set(weather.Rain) })
}
