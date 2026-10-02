package world

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
)

// moments hands the rules of the clock its Moment once every step of the simulation, just before
// the effects' pass: what happens when — every day at an hour, every winter — is a rule of a
// clock.Moment holding clock.Every or clock.At. First it puts on the world what was Applied to it
// and takes off what was Dispelled.
type moments struct {
	clock   *clock.Clock
	host    rule.ListHost[clock.Moment]
	tick    rule.TickSource
	applies *control.Queue[Apply]
	dispels *control.Queue[Dispel]
	last    time.Duration
	begun   bool
}

func (m *moments) system() goke.System {
	return goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, d time.Duration) {
		m.applies.Drain(func(i control.Issued[Apply]) {
			if i.Command.Effect != (effect.Effect{}) {
				i.Command.Effect.Cast(cb, m.clock.Entity())
			}
		})
		m.dispels.Drain(func(i control.Issued[Dispel]) {
			if i.Command.Effect != (effect.Effect{}) {
				i.Command.Effect.Dispel(m.clock.Entity())
			}
		})
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
