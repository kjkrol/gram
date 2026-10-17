package world

import (
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
)

var _ goke.System = (*despawnSystem)(nil)

type despawnSystem struct {
	w *module
	// one gone already is not despawned again
	alive *goke.Query
	base  goke.Comp[Base]
}

func newDespawnSystem(w *module) *despawnSystem { return &despawnSystem{w: w} }

func (s *despawnSystem) Init(si *goke.SysInit) { s.alive = si.NewQueryBuilder(&s.base).Build() }

func (s *despawnSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.w.despawns.Drain(func(i command.Issued[Despawn]) {
		if i.ByEntity && s.alive.Seek(i.Entity) {
			s.w.despawn(cb, i.Entity)
		}
	})
}
