package hooks

import (
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world/rule"
)

// Chase has every Predator steer at the nearest Prey it sees; seeing none, it turns a quarter
// aside every lookEvery. Hook it on the vision plugin.
func Chase(tags Tags, lookEvery time.Duration) plugin.Rule {
	started := time.Now()
	return rule.On("vision.chase", rule.Between(tags.Predator, tags.Prey), func(m *rule.Moment[vision.Sighting]) rule.Step {
		return m.Call(func(t plugin.Tick, s vision.Sighting) { hunt(t, s, started, lookEvery) })
	})
}

// hunt steers s's observer at the nearest prey shown, or a quarter aside when it is time to look.
func hunt(t plugin.Tick, s vision.Sighting, started time.Time, lookEvery time.Duration) {
	if s.Steering == nil {
		return
	}
	if len(s.Seen) > 0 {
		chase(s)
		return
	}
	if lookEvery > 0 && looksDue(started, t, lookEvery) {
		lookAside(s)
	}
}

// chase points the predator at the nearest prey — Seen comes nearest first.
func chase(s vision.Sighting) {
	ox, oy := centre(&s.Base.Pos)
	tx, ty := centre(&s.Seen[0].Base.Pos)
	if tx != ox || ty != oy {
		s.Steering.Request(geom.NewVec(tx-ox, ty-oy))
	}
}

// looksDue reports the tick in which another stretch of every has run out.
func looksDue(started time.Time, t plugin.Tick, every time.Duration) bool {
	now := t.Now.Sub(started)
	return now/every != (now-t.Dt)/every
}

// lookAside turns the predator a quarter to whichever side the coin falls.
func lookAside(s vision.Sighting) {
	heading := s.Base.Vel.Dir
	if heading.X == 0 && heading.Y == 0 {
		return
	}
	side := geom.NewVec(-heading.Y, heading.X)
	if rand.IntN(2) == 0 {
		side = geom.NewVec(heading.Y, -heading.X)
	}
	s.Steering.Request(side)
}
