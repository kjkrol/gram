package atmosphere

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
