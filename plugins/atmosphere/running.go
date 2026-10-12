package atmosphere

import "github.com/kjkrol/gram/plugins/atmosphere/climate"

// Running is which of the atmosphere's workings go on, each to switch off to see the rest without
// it. None of it is saved.
type Running struct {
	// Day: the sun and the moon cross the sky with the calendar; off, the light stands.
	Day bool
	// Weather: one weather follows another; off, the weather now stays.
	Weather bool
	// Wind: the wind blows; off, the air stands still.
	Wind bool
	// Clouds: the clouds cover the sky and shade the ground.
	Clouds bool
	// Falls: rain and snow fall.
	Falls bool
	// Weathering: the weather works on the board.
	Weathering bool
	// Stars: the stars come out at night.
	Stars bool
	// Moon: the moon shows on the sky and lights the night.
	Moon bool
}

// AllRunning is Running with all of it going on, the default.
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

// Running is which of the atmosphere's workings go on now.
func (p *Plugin) Running() Running {
	r := p.running
	r.Day = !p.sky.Frozen()
	return r
}
