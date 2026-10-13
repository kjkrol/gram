package clock

import (
	"time"

	"github.com/kjkrol/gram/rule/cond"
	"github.com/kjkrol/uid"
)

// Moment is one step of the simulation on the clock's time, from Last to Now: what a rule of the
// clock fires on, once every step. It is the moment of the clock's own entity, Clock, where an
// effect a rule applies lands.
type Moment struct {
	Last, Now time.Duration
	Clock     uid.UID64
}

// Who is the clock's entity: whose moment it is, for a rule.
func (m Moment) Who() uid.UID64 { return m.Clock }

// At is the word of the step the clock's time reaches at in.
func At(at time.Duration) cond.Word[Moment] {
	return cond.NewWord("at", func(m Moment) bool { return m.Last < at && at <= m.Now })
}

// Every is the word of each step the clock's time reaches another period in, the first at
// offset: a calendar's every day at an hour is Every(calendar.Daily(hour)), every winter
// Every(calendar.Seasonal(Winter)).
func Every(period, offset time.Duration) cond.Word[Moment] {
	if period <= 0 {
		panic("clock: Every needs a period above zero")
	}
	return cond.NewWord("every", func(m Moment) bool {
		if m.Now < offset {
			return false
		}
		before := (m.Last - offset) / period
		if m.Last < offset {
			before = -1
		}
		return (m.Now-offset)/period > before
	})
}
