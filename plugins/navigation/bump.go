package navigation

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/collision"
)

// bumps is how navigation answers what a unit under orders struck, as collision recorded it on
// its Collider: not at all without collision, any contact under CellSpacing, the solid ground
// under BodySpacing (units struck are a Touch, for the rules).
type bumps uint8

const (
	noBumps bumps = iota
	anyBump
	groundBump
)

// bump marks every unit under orders Bumped by what it struck this step: under anyBump whatever it
// struck, unless it still holds the route a bump gave it; under groundBump the solid ground, with
// the way off it, every step it touches it.
func (s *navigationSystem) bump() {
	if s.bumps == noBumps {
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
			if s.bumps == anyBump {
				if o.Cooldown == 0 {
					o.Bumped = true
				}
				continue
			}
			struck(o, contacts)
		}
	}
}

// struck marks o Bumped with the way off the solid ground among contacts.
func struck(o *MoveOrder, contacts []collision.Contact) {
	var n geom.Vec
	for _, c := range contacts {
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
