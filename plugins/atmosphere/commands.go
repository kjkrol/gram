package atmosphere

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
)

// Freeze stops the light of the day at the hour it stands, or lets a frozen light go with the
// calendar again: only the light — the calendar and the weather go on — and not saved. P by
// default.
type Freeze struct{}

// Later moves a frozen light half an hour on; it does nothing while the light goes with the
// calendar. Shift+] by default.
type Later struct{}

// Earlier moves a frozen light half an hour back. Shift+[ by default.
type Earlier struct{}

// ChangeWeather has the weather go on to its next state now, thrown as when one runs out.
// Shift+W by default, with the camera free.
type ChangeWeather struct{}

// SetWeather has the weather go into the state called Name now (weather.Clear, weather.Storm, a
// game's own); a name the climate lacks changes nothing.
type SetWeather struct{ Name string }

// Queues are where the atmosphere's commands land.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.freezes, &p.laters, &p.earliers, &p.changes, &p.sets}
}

// DefaultBindings: P freezes the light, Shift+] and Shift+[ move a frozen light half an hour,
// Shift+W changes the weather — with the camera free: riding in a unit, W with Shift sprints.
func (p *Plugin) DefaultBindings() []control.Binding {
	shift := control.Mods{Shift: true}
	return []control.Binding{
		control.Command(control.KeyPress{Key: control.KeyP}, "Freeze the light of the day", func(control.Context) (Freeze, bool) { return Freeze{}, true }),
		control.Command(control.KeyPress{Key: control.KeyBracketRight, Mods: shift}, "Frozen light half an hour later", func(control.Context) (Later, bool) { return Later{}, true }),
		control.Command(control.KeyPress{Key: control.KeyBracketLeft, Mods: shift}, "Frozen light half an hour earlier", func(control.Context) (Earlier, bool) { return Earlier{}, true }),
		control.Command(control.KeyPress{Key: control.KeyW, Mods: shift}, "Change the weather", func(control.Context) (ChangeWeather, bool) { return ChangeWeather{}, true }).In(camera.Outside),
	}
}

// carryOut carries out the commands given since the last tick: the light's at once, the
// weather's at the next step of the simulation.
func (p *Plugin) carryOut() {
	p.freezes.Drain(func(control.Issued[Freeze]) { p.sky.SetFrozen(!p.sky.Frozen()) })
	p.laters.Drain(func(control.Issued[Later]) { p.sky.Shift(sky.HalfHour) })
	p.earliers.Drain(func(control.Issued[Earlier]) { p.sky.Shift(-sky.HalfHour) })
	p.changes.Drain(func(control.Issued[ChangeWeather]) { p.climate.Change() })
	p.sets.Drain(func(i control.Issued[SetWeather]) { p.climate.Set(i.Command.Name) })
}
