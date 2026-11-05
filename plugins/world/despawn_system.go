package world

import (
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/sector"
)

var _ goke.System = (*despawnSystem)(nil)

// carries out what an entity gives itself about itself: its Keen and Calm, then its Despawn
type despawnSystem struct {
	w *module
	// one gone already is not despawned again
	alive  *goke.Query
	base   goke.Comp[Base]
	keenID goke.CompID
}

func newDespawnSystem(w *module) *despawnSystem { return &despawnSystem{w: w} }

func (s *despawnSystem) Init(si *goke.SysInit) {
	s.alive = si.NewQueryBuilder(&s.base).Build()
	s.keenID = si.RegComp[sector.Keen]()
}

func (s *despawnSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.w.keens.Drain(func(i command.Issued[Keen]) {
		if i.ByEntity && s.alive.Seek(i.Entity) {
			cb.AddOne(i.Entity, s.keenID, sector.Keen{Level: i.Command.Level})
		}
	})
	s.w.calms.Drain(func(i command.Issued[Calm]) {
		if i.ByEntity && s.alive.Seek(i.Entity) {
			cb.RemoveCompOne(i.Entity, s.keenID)
		}
	})
	s.w.despawns.Drain(func(i command.Issued[Despawn]) {
		if i.ByEntity && s.alive.Seek(i.Entity) {
			s.w.despawn(cb, i.Entity)
		}
	})
}
