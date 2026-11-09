package world

import (
	"time"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*despawnSystem)(nil)

// carries out what an entity gives itself about itself: its Keen and Calm, then its Despawn; and
// a Whole's handle's despawn on its pieces
type despawnSystem struct {
	w *module
	// one gone already is not despawned again
	alive  *goke.Query
	base   goke.Comp[Base]
	whole  goke.OptComp[Whole]
	keenID goke.CompID
	// every piece, walked for those of a handle going
	pieces  *goke.Query
	pieceOf goke.Comp[PieceOf]
	going   []uid.UID64
}

func newDespawnSystem(w *module) *despawnSystem { return &despawnSystem{w: w} }

func (s *despawnSystem) Init(si *goke.SysInit) {
	s.alive = si.NewQueryBuilder(&s.base).Optional(&s.whole).Build()
	s.keenID = si.RegComp[sector.Keen]()
	s.pieces = si.NewQueryBuilder(&s.pieceOf).Build()
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

// despawns through cb the pieces of id where it is a Whole's handle, with it
func (s *despawnSystem) cascade(cb *goke.CmdBuf, id uid.UID64) {
	if s.alive == nil || !s.alive.Seek(id) || !s.whole.Present(s.alive.Cursor()) {
		return
	}
	from := len(s.going)
	for s.pieces.All(); s.pieces.Next(); {
		cur := s.pieces.Cursor()
		for i, p := range s.pieceOf.Slice(cur) {
			if p.Whole == id {
				s.going = append(s.going, cur.IDs[i])
			}
		}
	}
	// a piece that is a handle itself despawns its own past to, s.going growing meanwhile
	to := len(s.going)
	for k := from; k < to; k++ {
		s.w.despawn(cb, s.going[k])
	}
	s.going = s.going[:from]
}
