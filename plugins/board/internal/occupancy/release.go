package occupancy

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// ReleaseSystem has o let go, every step, of the holds of whoever is no longer in the world, so
// one despawned — fallen in, say — blocks no cell.
func ReleaseSystem(o cell.Occupancy) goke.System {
	s := &releaseSystem{occupancy: o}
	s.gone = s.isGone
	return s
}

var _ goke.System = (*releaseSystem)(nil)

type releaseSystem struct {
	occupancy cell.Occupancy
	alive     *goke.Query // who is still in the world
	base      goke.Comp[world.Base]
	gone      func(uid.UID64) bool // isGone, bound once
}

func (s *releaseSystem) Init(si *goke.SysInit) { s.alive = si.NewQueryBuilder(&s.base).Build() }

// isGone reports whether id is no longer in the world.
func (s *releaseSystem) isGone(id uid.UID64) bool { return !s.alive.Seek(id) }

func (s *releaseSystem) Update(*goke.CmdBuf, time.Duration) {
	if s.occupancy != nil {
		s.occupancy.Release(s.gone)
	}
}
