package board

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world"
)

var _ goke.System = (*altitudeSystem)(nil)

// altitudeSystem puts every mover carrying a Z at the ground under its centre plus its Lift, every
// tick, after movement and collisions; a Quasi3D world's system alone.
type altitudeSystem struct {
	brd *Board

	query *goke.Query
	base  goke.Comp[world.Base]
	z     goke.Comp[world.Z]
	mover goke.Comp[Mover]
}

func newAltitudeSystem(brd *Board) *altitudeSystem { return &altitudeSystem{brd: brd} }

func (s *altitudeSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base, &s.z, &s.mover).Build()
}

func (s *altitudeSystem) Update(*goke.CmdBuf, time.Duration) {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		bases, zs, movers := s.base.Slice(cursor), s.z.Slice(cursor), s.mover.Slice(cursor)
		for i := range cursor.IDs {
			zs[i].Altitude = s.brd.GroundAt(Center(bases[i].Pos)) + movers[i].Lift
		}
	}
}
