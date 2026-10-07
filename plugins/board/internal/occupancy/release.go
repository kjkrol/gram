package occupancy

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// ReleaseSystem keeps o true to the world: as it starts — after a Populate as after a Load — it
// has every unit hold the cell it stands on (unit.At, in its Mover's domains), and every step it
// lets go of the holds of whoever is no longer in the world, so one despawned — fallen in, say —
// blocks no cell.
func ReleaseSystem(o cell.Occupancy) goke.System {
	s := &releaseSystem{occupancy: o}
	s.gone = s.isGone
	return s
}

var _ goke.System = (*releaseSystem)(nil)

type releaseSystem struct {
	occupancy cell.Occupancy
	alive     *goke.Query // who is still in the world, and where the units stand
	base      goke.Comp[world.Base]
	at        goke.OptComp[unit.At]
	mover     goke.OptComp[unit.Mover]
	gone      func(uid.UID64) bool // isGone, bound once
}

func (s *releaseSystem) Init(si *goke.SysInit) {
	s.alive = si.NewQueryBuilder(&s.base).Optional(&s.at).Optional(&s.mover).Build()
	if s.occupancy == nil {
		return
	}
	for s.alive.All(); s.alive.Next(); {
		cur := s.alive.Cursor()
		cells, movers := s.at.Slice(cur), s.mover.Slice(cur)
		for i, id := range cur.IDs {
			if cells != nil {
				s.occupancy.Enter(cells[i].Cell, id, unit.DomainAt(movers, i))
			}
		}
	}
}

// isGone reports whether id is no longer in the world.
func (s *releaseSystem) isGone(id uid.UID64) bool { return !s.alive.Seek(id) }

func (s *releaseSystem) Update(*goke.CmdBuf, time.Duration) {
	if s.occupancy != nil {
		s.occupancy.Release(s.gone)
	}
}
