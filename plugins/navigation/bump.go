package navigation

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/uid"
)

// bumped is the Struck trigger navigation registers on collision: an entity under orders that
// struck someone is marked Bumped, unless it is still holding the route a bump gave it.
func bumped() plugin.Trigger {
	return act.Trigger[collision.Struck]("hook").RunsOn(func(_ plugin.Tick, o *MoveOrder, s collision.Struck) {
		if len(s.Contacts) > 0 && o.Cooldown == 0 {
			o.Bumped = true
		}
	})
}

// struckBy is the Struck trigger navigation registers on collision under BodySpacing: an entity
// under orders that struck someone, or the solid ground, is marked Bumped, with the way off them
// and whom it struck, every tick it touches them.
func struckBy() plugin.Trigger {
	return act.Trigger[collision.Struck]("hook").RunsOn(func(_ plugin.Tick, o *MoveOrder, s collision.Struck) {
		if len(s.Contacts) == 0 {
			return
		}
		var n geom.Vec
		var hit uid.UID64
		unit := false
		for _, c := range s.Contacts {
			n = geom.NewVec(n.X+c.Normal.X, n.Y+c.Normal.Y)
			if !unit && !c.Terrain {
				hit, unit = c.Other, true
			}
		}
		if math.Hypot(n.X, n.Y) < 1e-9 {
			n = s.Contacts[0].Normal // squeezed from both sides: off the first
		}
		l := math.Hypot(n.X, n.Y)
		if l < 1e-9 {
			return // no way off it told: nothing to answer
		}
		o.Bumped, o.Struck, o.Hit, o.HitUnit = true, geom.NewVec(n.X/l, n.Y/l), hit, unit
	})
}
