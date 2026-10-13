package atmosphere

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
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

func (Freeze) Command()        {}
func (Later) Command()         {}
func (Earlier) Command()       {}
func (ChangeWeather) Command() {}
func (SetWeather) Command()    {}

var (
	_ command.Command = Freeze{}
	_ command.Command = Later{}
	_ command.Command = Earlier{}
	_ command.Command = ChangeWeather{}
	_ command.Command = SetWeather{}
)

// Queues are where the atmosphere's commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.freezes, &p.laters, &p.earliers, &p.changes, &p.sets}
}

// DefaultBindings are P (Freeze), Shift+] and Shift+[ (Later, Earlier) and, with the camera
// free, Shift+W (ChangeWeather).
func (p *Plugin) DefaultBindings() []control.Binding {
	shift := control.Mods{Shift: true}
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyP}, "Freeze the light of the day", func(control.Context) (Freeze, bool) { return Freeze{}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketRight, Mods: shift}, "Frozen light half an hour later", func(control.Context) (Later, bool) { return Later{}, true }),
		control.Build(control.KeyPress{Key: control.KeyBracketLeft, Mods: shift}, "Frozen light half an hour earlier", func(control.Context) (Earlier, bool) { return Earlier{}, true }),
		control.Build(control.KeyPress{Key: control.KeyW, Mods: shift}, "Change the weather", func(control.Context) (ChangeWeather, bool) { return ChangeWeather{}, true }).In(camera.Outside),
	}
}

func (p *Plugin) carryOut() {
	p.freezes.Drain(func(command.Issued[Freeze]) { p.sky.SetFrozen(!p.sky.Frozen()) })
	p.laters.Drain(func(command.Issued[Later]) { p.sky.Shift(sky.HalfHour) })
	p.earliers.Drain(func(command.Issued[Earlier]) { p.sky.Shift(-sky.HalfHour) })
	p.changes.Drain(func(command.Issued[ChangeWeather]) { p.climate.Change() })
	p.sets.Drain(func(i command.Issued[SetWeather]) { p.climate.Set(i.Command.Name) })
}
