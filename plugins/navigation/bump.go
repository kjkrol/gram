package navigation

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/collision"
)

// bumped is the Struck hook navigation hangs on collision, its own: an entity under orders that
// struck someone is marked Bumped, unless it is still holding the route a bump gave it.
func bumped() plugin.Rule {
	return host.Each(func(_ plugin.Tick, o *MoveOrder, s collision.Struck) {
		if len(s.Contacts) > 0 && o.Cooldown == 0 {
			o.Bumped = true
		}
	})
}

// struckBy is the Struck hook navigation hangs on collision under BodySpacing, its own: an entity
// under orders that struck the solid ground is marked Bumped, with the way off it, every tick it
// touches it. Units struck are a Touch, for the rules.
func struckBy() plugin.Rule { return host.Each(struck) }

// struck marks o Bumped with the way off the solid ground s struck.
func struck(_ plugin.Tick, o *MoveOrder, s collision.Struck) {
	var n geom.Vec
	for _, c := range s.Contacts {
		if c.Terrain {
			n = geom.NewVec(n.X+c.Normal.X, n.Y+c.Normal.Y)
		}
	}
	l := math.Hypot(n.X, n.Y)
	if l < 1e-9 {
		return // no ground struck, or squeezed from both sides: nothing to answer
	}
	o.Bumped, o.Struck, o.Hit, o.HitUnit = true, geom.NewVec(n.X/l, n.Y/l), 0, false
}
