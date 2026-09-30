package effect

import (
	"time"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world/clock"
)

// moments hands the triggers of the clock its Moment once every step of the simulation, with the
// effects' pass: what used to be a schedule — every day at an hour, every winter — is a
// act.Trigger[clock.Moment] holding clock.Every or clock.At.
type moments struct {
	clock *clock.Clock
	host  host.ListHost[clock.Moment]
	last  time.Duration
	begun bool
}

// run fires the triggers on the step ending now: from where the last one ended.
func (m *moments) run(t plugin.Tick) {
	now := m.clock.Time() + t.Dt
	if !m.begun {
		m.last, m.begun = m.clock.Time(), true
	}
	if !m.host.Empty() {
		m.host.Run(t, clock.Moment{Last: m.last, Now: now})
	}
	m.last = now
}
