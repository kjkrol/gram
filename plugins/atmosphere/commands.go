package atmosphere

import (
	"fmt"
	"math"
	"slices"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/atmosphere/climate/weather"
)

// Freeze stops the light of the day at the hour it stands (Settings.Frozen), or lets a frozen
// light go again. Only the light stops; the calendar and the weather go on. The settings screen
// alone.
type Freeze struct{ On bool }

// Hour freezes the light At a part of the day, 0 to 1 round midnight (Settings.Frozen,
// Settings.Hour): the hour taken, Freeze{On: false} lets the day go on from there. The settings
// screen alone.
type Hour struct{ At float32 }

// ChangeWeather has the weather go on to its next state now.
type ChangeWeather struct{}

// SetWeather has the weather go into the state called Name now (weather.Clear, weather.Storm, a
// game's own); a name the climate lacks changes nothing.
type SetWeather struct{ Name string }

// Clouds has the clouds drawn in the Look (CloudLook, Settings.Clouds). The settings screen
// alone.
type Clouds struct{ Look CloudLook }

// GroundShade has the clouds shade the ground, or not (CloudShadows.Ground); FigureShade the
// figures (CloudShadows.Figures). The settings screen alone.
type GroundShade struct{ On bool }

type FigureShade struct{ On bool }

func (GroundShade) Command() {}
func (FigureShade) Command() {}

var (
	_ command.Command = GroundShade{}
	_ command.Command = FigureShade{}
)

// CloudReach has the clouds drawn as bodies out to Share of the camera's sight from where it
// looks and flat beyond (Settings.CloudReach), 1 everywhere. The settings screen alone.
type CloudReach struct{ Share float32 }

func (CloudReach) Command() {}

var _ command.Command = CloudReach{}

// CloudsDrawnAt has the clouds drawn at a Coarse of the pixels each way, 4, 6 or 8
// (Settings.CloudCoarse): they are soft, so a quarter of them loses nothing to the eye, a
// sixth or an eighth little, for a frame the cheaper. The settings screen alone.
type CloudsDrawnAt struct{ Coarse int }

func (CloudsDrawnAt) Command() {}

var _ command.Command = CloudsDrawnAt{}

// AdaptClouds has the clouds follow the cameras' frame budget, or not (Settings.AdaptiveClouds):
// the atmosphere's Write is a cameras.Lever (cameras.Config.Levers), the clouds drawn at a sixth of
// the pixels, then an eighth, then as bodies to half the sight alone as the rate falls under the
// budget, and back up as it stays over; the look stays as set. The settings screen alone.
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
	CloudReach     float32 // of the camera's sight the clouds are bodies out to; zero, all
	CloudCoarse    int     // of the pixels each way the clouds are drawn at; zero, 4
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
		&p.shades, &p.groundShades, &p.figureShades, &p.reaches, &p.coarses, &p.adapts,
		&p.downpours}
}

// DefaultBindings are none: the light, the clouds and the weather are the settings screen's.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }

// Settings are the frozen light and the hour, the clouds' look, pixels, reach and shadows, and
// the weather (plugin.Tunable). The hour shows the day's time, and sets it frozen.
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
			Label: "Hour",
			Min:   0,
			Max:   23.5,
			Step:  0.5,
			Get:   func() float64 { return float64(p.lightsHour()) * 24 },
			Set:   func(v float64) command.Command { return Hour{At: float32(v / 24)} },
			Shows: func(v float64) string { return fmt.Sprintf("%02d:%02d", int(v), int(v*60)%60) },
		},
		control.Choice{
			Label:   "Clouds",
			Options: looks,
			Get:     func() int { return int(p.read.ShownClouds()) },
			Set:     func(i int) command.Command { return Clouds{Look: CloudLook(i)} },
		},
		control.Choice{
			Label:   "Clouds drawn at",
			Options: []string{"a quarter of the pixels", "a sixth", "an eighth"},
			Get:     func() int { return coarseRung(p.read.CloudsDrawnAt()) },
			Set:     func(i int) command.Command { return CloudsDrawnAt{Coarse: coarses[i]} },
		},
		control.Range{
			Label: "Clouds as bodies out to",
			Min:   10,
			Max:   100,
			Step:  10,
			Get:   func() float64 { return math.Round(float64(p.read.CloudsReach()) * 100) },
			Set:   func(v float64) command.Command { return CloudReach{Share: float32(v / 100)} },
			Shows: func(v float64) string { return fmt.Sprintf("%.0f%% of the sight", v) },
		},
		control.Switch{
			Label: "Clouds follow the frame budget",
			Get:   p.read.CloudsAdaptive,
			Set:   func(on bool) command.Command { return AdaptClouds{On: on} },
		},
		control.Switch{
			Label: "Clouds shade the ground",
			Get:   func() bool { return p.shadows.Ground },
			Set:   func(on bool) command.Command { return GroundShade{On: on} },
		},
		control.Switch{
			Label: "Clouds shade the figures",
			Get:   func() bool { return p.shadows.Figures },
			Set:   func(on bool) command.Command { return FigureShade{On: on} },
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
	p.hours.Drain(func(i command.Issued[Hour]) {
		p.sky.SetFrozen(true)
		p.sky.SetHour(i.Command.At)
	})
	p.changes.Drain(func(command.Issued[ChangeWeather]) { p.climate.Change() })
	p.sets.Drain(func(i command.Issued[SetWeather]) { p.climate.Set(i.Command.Name) })
	p.reaches.Drain(func(i command.Issued[CloudReach]) { p.write.ReachClouds(i.Command.Share) })
	p.coarses.Drain(func(i command.Issued[CloudsDrawnAt]) { p.write.DrawCloudsAt(i.Command.Coarse) })
	p.adapts.Drain(func(i command.Issued[AdaptClouds]) { p.write.AdaptClouds(i.Command.On) })
	p.clouds.Drain(func(i command.Issued[Clouds]) { p.write.ShowClouds(i.Command.Look) })
	p.shades.Drain(func(i command.Issued[HighShade]) { p.shadeHigh(i.Command.On) })
	p.groundShades.Drain(func(i command.Issued[GroundShade]) { p.shadows.Ground = i.Command.On })
	p.figureShades.Drain(func(i command.Issued[FigureShade]) { p.shadows.Figures = i.Command.On })
	p.downpours.Drain(func(command.Issued[Downpour]) { p.climate.Set(weather.Rain) })
	p.Res.Settings.Frozen, p.Res.Settings.Hour = p.sky.Frozen(), p.sky.Hour()
}

// lightsHour is the time of day the light stands at: the frozen light's, else the calendar's
func (p *Plugin) lightsHour() float32 {
	if p.sky.Frozen() {
		return p.sky.Hour()
	}
	return p.calendar.Now().Time
}
