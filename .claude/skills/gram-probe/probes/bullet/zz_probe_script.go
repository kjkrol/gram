package main

import (
	"math/rand/v2"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/bullet"
)

// from the second second on, the soldier shoots a round every 20 ticks and throws a grenade
// every 90, at a point within eight cells
func (p *probe) play() {
	if p.tick < 120 {
		return
	}
	round, grenade := p.tick%20 == 0, p.tick%90 == 0
	if !round && !grenade {
		return
	}
	soldier, _, _, ok := p.a.world.Kinds().Lookup(SoldierKind)
	if !ok {
		return
	}
	ammo := p.a.bullet.Shots().Named(RoundKind)
	if grenade {
		ammo = p.a.bullet.Shots().Named(GrenadeKind)
	}
	for p.q.All(); p.q.Next(); {
		cur := p.q.Cursor()
		for i, b := range p.base.Slice(cur) {
			if b.TypeID != soldier {
				continue
			}
			c := b.Pos.Center()
			at := geom.NewVec(c.X+(rand.Float64()*2-1)*8*CellSize, c.Y+(rand.Float64()*2-1)*8*CellSize)
			p.a.world.Write().Carrier().PutFrom(cur.IDs[i], bullet.Shoot{Ammo: ammo, At: at, Targeted: true})
		}
	}
}
