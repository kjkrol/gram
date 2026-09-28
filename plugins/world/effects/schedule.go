package effects

import (
	"time"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world/clock"
)

// Schedule is what happens when, on the clock's time: entries that come once at a moment or every
// period, each running what it was given — casting effects, granting a phase to the clock — with
// the simulation's step in which its moment falls. Entries are laid at Init, in code, and fire by
// the clock's time alone, so a loaded game goes on from where it was: whatever came before the
// save came then.
type Schedule struct {
	clock   *clock.Clock
	entries []entry
	last    time.Duration
	begun   bool
}

// entry is one thing to do: at a moment, or every period after offset.
type entry struct {
	at     time.Duration
	every  time.Duration
	offset time.Duration
	do     func(t plugin.Tick)
}

// At has do run in the step of the simulation when the clock reaches at.
func (s *Schedule) At(at time.Duration, do func(t plugin.Tick)) {
	s.entries = append(s.entries, entry{at: at, do: do})
}

// Every has do run every period of the clock's time, first at offset: a calendar's every day at
// an hour is Every(day, hour), every winter Every(year, when winter starts).
func (s *Schedule) Every(period, offset time.Duration, do func(t plugin.Tick)) {
	if period <= 0 {
		panic("effects: a Schedule entry every period needs a period above zero")
	}
	s.entries = append(s.entries, entry{every: period, offset: offset, do: do})
}

// run fires every entry whose moment falls in the step ending now, once each.
func (s *Schedule) run(t plugin.Tick) {
	now := s.clock.Time() + t.Dt
	if !s.begun {
		s.last, s.begun = s.clock.Time(), true
	}
	for _, e := range s.entries {
		if e.fires(s.last, now) {
			e.do(t)
		}
	}
	s.last = now
}

// fires reports whether the entry's moment falls in (last, now].
func (e entry) fires(last, now time.Duration) bool {
	if e.every == 0 {
		return last < e.at && e.at <= now
	}
	if now < e.offset {
		return false
	}
	before := (last - e.offset) / e.every
	if last < e.offset {
		before = -1
	}
	return (now-e.offset)/e.every > before
}
