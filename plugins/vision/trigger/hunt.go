package trigger

import (
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world/act"
)

// Chase steers at the nearest entity shown — the body of a trigger between Predator and Prey, so it is
// shown prey alone; shown none, it turns a quarter aside every lookEvery.
func Chase(lookEvery time.Duration) act.Instant {
	started := time.Now()
	var r act.Reaction[vision.Sighting]
	return r.Run(func(t plugin.Tick, s vision.Sighting) {
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
	})
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
