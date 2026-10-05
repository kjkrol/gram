package world

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugin"
)

// moments hands the rules of the clock its Moment once every step of the simulation, just before
// the effects' pass: what happens when — every day at an hour, every winter — is a rule of a
// clock.Moment holding clock.Every or clock.At.
type moments struct {
	clock *clock.Clock
	host  plugin.StepRules[clock.Moment]
	tick  plugin.TickSource
	last  time.Duration
	begun bool
}

func (m *moments) system() goke.System {
	return goke.SystemFn{OnInit: func(*goke.SysInit) { m.host.Bind() }, OnUpdate: func(cb *goke.CmdBuf, d time.Duration) {
		now := m.clock.Time() + d
		if !m.begun {
			m.last, m.begun = m.clock.Time(), true
		}
		if !m.host.Empty() {
			m.host.Run(m.tick.Of(cb, d), clock.Moment{Last: m.last, Now: now, Clock: m.clock.Entity()})
		}
		m.last = now
	}}
}
