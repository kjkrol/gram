package clock

import "time"

// Moment is one step of the simulation on the clock's time, from Last to Now: what a trigger of
// the clock fires on, once every step, so a loaded game goes on from where it was.
type Moment struct{ Last, Now time.Duration }

// At holds in the step the clock's time reaches at.
func At(at time.Duration) func(Moment) bool {
	return func(m Moment) bool { return m.Last < at && at <= m.Now }
}

// Every holds in each step the clock's time reaches another period in, the first at offset: a
// calendar's every day at an hour is Every(calendar.Daily(hour)), every winter
// Every(calendar.Seasonal(Winter)).
func Every(period, offset time.Duration) func(Moment) bool {
	if period <= 0 {
		panic("clock: Every needs a period above zero")
	}
	return func(m Moment) bool {
		if m.Now < offset {
			return false
		}
		before := (m.Last - offset) / period
		if m.Last < offset {
			before = -1
		}
		return (m.Now-offset)/period > before
	}
}
