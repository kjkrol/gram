package traffic

import (
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/collision"
)

type Bumps uint8

const (
	noBumps Bumps = iota
	AnyBump
	GroundBump
)

func (s *System) bump() {
	if s.Bumps == noBumps {
		return
	}
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		orders, colls := s.order.Slice(cursor), s.coll.Slice(cursor)
		if orders == nil || colls == nil {
			continue
		}
		for i := range cursor.IDs {
			contacts := colls[i].Contacts()
			if len(contacts) == 0 {
				continue
			}
			o := &orders[i]
			if s.Bumps == AnyBump {
				if o.Cooldown == 0 && struckSomething(contacts) {
					o.Bumped = true
				}
				continue
			}
			struck(o, contacts)
		}
	}
}

func struck(o *order.MoveOrder, contacts []collision.Contact) {
	var n geom.Vec
	for _, c := range contacts {
		if c.Terrain {
			n = geom.NewVec(n.X+c.Normal.X, n.Y+c.Normal.Y)
		}
	}
	l := math.Hypot(n.X, n.Y)
	if l < 1e-9 {
		return // no ground struck, or squeezed from both sides
	}
	o.Bumped, o.Struck, o.Hit, o.HitUnit = true, geom.NewVec(n.X/l, n.Y/l), 0, false
}

// a sensor, a shot, blocks nobody; nor does what a unit merely rests against
func struckSomething(contacts []collision.Contact) bool {
	for _, c := range contacts {
		if !c.Sensed && !c.Resting {
			return true
		}
	}
	return false
}
