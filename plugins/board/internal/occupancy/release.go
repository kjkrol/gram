package occupancy

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// ReleaseSystem keeps o true to the world: as it starts — after a Populate as after a Load — it
// has every unit hold the cell it stands on (unit.At, in its Mover's domains) on the level onDeck
// says, and every step it lets go of the holds of whoever is no longer in the world, so one
// despawned — fallen in, say — blocks no cell, and holds the cell of every unit standing under a
// raised deck on the level its feet are on, as it climbs onto the deck or falls off it.
func ReleaseSystem(o cell.Occupancy, onDeck OnDeck) goke.System {
	s := &releaseSystem{occupancy: o, onDeck: onDeck}
	s.gone = s.isGone
	return s
}

// OnDeck reports whether whoever stands at p, its feet at feet, stepping reach high, stands in c
// on the raised deck over it, and whether c has one at all.
type OnDeck func(c cell.ID, p geom.Vec, feet, reach float64) (deck, raised bool)

var _ goke.System = (*releaseSystem)(nil)

type releaseSystem struct {
	occupancy cell.Occupancy
	onDeck    OnDeck
	alive     *goke.Query // who is still in the world, and where the units stand
	base      goke.Comp[world.Base]
	at        goke.OptComp[unit.At]
	mover     goke.OptComp[unit.Mover]
	z         goke.OptComp[world.Z]
	gone      func(uid.UID64) bool // isGone, bound once
}

func (s *releaseSystem) Init(si *goke.SysInit) {
	s.alive = si.NewQueryBuilder(&s.base).Optional(&s.at).Optional(&s.mover).Optional(&s.z).
		Build()
	if s.occupancy != nil {
		s.hold(true)
	}
}

// hold has every unit hold its cell on the level its feet are on: all of them, or only those under
// a raised deck.
func (s *releaseSystem) hold(all bool) {
	for s.alive.All(); s.alive.Next(); {
		cur := s.alive.Cursor()
		cells, movers, bases, zs := s.at.Slice(cur), s.mover.Slice(cur), s.base.Slice(cur),
			s.z.Slice(cur)
		if cells == nil {
			continue
		}
		for i, id := range cur.IDs {
			feet, reach := math.Inf(-1), 0.0
			if zs != nil {
				feet, reach = zs[i].Altitude, zs[i].Reach()
			}
			deck, raised := s.onDeck(cells[i].Cell, bases[i].Pos.Center(), feet, reach)
			if all || raised {
				s.occupancy.Enter(cells[i].Cell, deck, id, unit.DomainAt(movers, i))
			}
		}
	}
}

// isGone reports whether id is no longer in the world.
func (s *releaseSystem) isGone(id uid.UID64) bool { return !s.alive.Seek(id) }

func (s *releaseSystem) Update(*goke.CmdBuf, time.Duration) {
	if s.occupancy != nil {
		s.occupancy.Release(s.gone)
		s.hold(false)
	}
}
