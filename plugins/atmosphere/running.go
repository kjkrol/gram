package atmosphere

import "github.com/kjkrol/gram/plugins/atmosphere/climate"

// Running is which of the atmosphere's workings go on, each to switch off to see the rest without
// it, at the start (Config.Running) or as the game goes (Plugin.SetRunning); none of it is saved.
type Running struct {
	// Day: the day goes on, the sun and the moon crossing the sky with the calendar; off, the
	// light stands at the hour it has, as P freezes it.
	Day bool
	// Weather: one weather follows another as the climate throws them; off, the weather now
	// stays, though Shift+W still changes it.
	Weather bool
	// Wind: the wind blows, carrying the clouds, swaying what sways, slanting the rain; off, the
	// air stands still.
	Wind bool
	// Clouds: the clouds cover the sky and shade the ground; off, the sky is clear.
	Clouds bool
	// Falls: rain and snow fall; off, nothing falls.
	Falls bool
	// Weathering: the weather works on the board, snow lying, water freezing; off, the ground
	// stays as it is.
	Weathering bool
	// Stars: the stars come out at night.
	Stars bool
	// Moon: the moon shows on the sky and lights the night.
	Moon bool
}

// AllRunning is Running with all of it going on: what an atmosphere has when its Config says
// nothing.
func AllRunning() Running {
	return Running{Day: true, Weather: true, Wind: true, Clouds: true, Falls: true, Weathering: true, Stars: true, Moon: true}
}

// SetRunning has the atmosphere's workings go on as r says, at once.
func (p *Plugin) SetRunning(r Running) {
	p.running = r
	p.sky.SetFrozen(!r.Day)
	p.sky.SetMoon(r.Moon)
	p.climate.SetRunning(climate.Running{Changes: r.Weather, Wind: r.Wind, Clouds: r.Clouds, Falls: r.Falls})
	if p.weathering != nil {
		p.weathering.SetRunning(r.Weathering)
	}
}

// Running is which of the atmosphere's workings go on now: the Day's as the light stands, frozen
// or not.
func (p *Plugin) Running() Running {
	r := p.running
	r.Day = !p.sky.Frozen()
	return r
}
