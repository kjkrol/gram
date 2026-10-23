package world

import (
	"time"

	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
)

type moments struct {
	clock *clock.Clock
	host  host.StepRules[clock.Moment]
	tick  host.TickSource
	last  time.Duration
	begun bool
}

func (m *moments) system() goke.System {
	return goke.SystemFn{OnInit: func(*goke.SysInit) { m.host.Bind() }, OnUpdate: func(cb *goke.CmdBuf, d time.Duration) {
		now := m.clock.Read().Time() + d
		if !m.begun {
			m.last, m.begun = m.clock.Read().Time(), true
		}
		if !m.host.Empty() {
			m.host.Run(m.tick.Of(d), clock.Moment{Last: m.last, Now: now, Clock: m.clock.Read().Entity()})
		}
		m.last = now
	}}
}
