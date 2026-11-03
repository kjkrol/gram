package gravity

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/motion"
)

var _ goke.System = (*giveSystem)(nil)

// before the simulation's steps, so whatever a body is told in them finds its drift there
type giveSystem struct {
	p         *Plugin
	driftless *goke.Query
	id        goke.CompID
}

func (s *giveSystem) Init(si *goke.SysInit) {
	s.id = si.RegComp[motion.Drift]()
	var body goke.Comp[Body]
	s.driftless = si.NewQueryBuilder(&body).Exclude(goke.Exclude[motion.Drift]()).
		With(s.p.worldPlugin.Pace(lod.Steady)).Build()
}

func (s *giveSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	for s.driftless.Paced(d); s.driftless.Next(); {
		for _, e := range s.driftless.Cursor().IDs {
			cb.AddOne(e, s.id, motion.Drift{})
		}
	}
}
