package gravity

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world"
)

var _ goke.System = (*giveSystem)(nil)

// giveSystem puts a world.Drift on every body without one, once a tick before the simulation's
// steps: whatever a body is told in them finds its drift there.
type giveSystem struct {
	driftless *goke.Query
	id        goke.CompID
}

func (s *giveSystem) Init(si *goke.SysInit) {
	s.id = si.RegComp[world.Drift]()
	var body goke.Comp[Body]
	s.driftless = si.NewQueryBuilder(&body).Exclude(goke.Exclude[world.Drift]()).Build()
}

func (s *giveSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	for s.driftless.All(); s.driftless.Next(); {
		for _, e := range s.driftless.Cursor().IDs {
			cb.AddOne(e, s.id, world.Drift{})
		}
	}
}
