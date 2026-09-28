package topography

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
)

var _ goke.System = (*altitudeSystem)(nil)

// altitudeSystem puts every mover carrying a Z at the ground under its centre plus its Lift, every
// step, after movement and collisions.
type altitudeSystem struct {
	relief *Relief

	query *goke.Query
	base  goke.Comp[world.Base]
	z     goke.Comp[world.Z]
	mover goke.Comp[board.Mover]
}

func newAltitudeSystem(r *Relief) *altitudeSystem { return &altitudeSystem{relief: r} }

func (s *altitudeSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base, &s.z, &s.mover).Build()
}

func (s *altitudeSystem) Update(*goke.CmdBuf, time.Duration) {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		bases, zs, movers := s.base.Slice(cursor), s.z.Slice(cursor), s.mover.Slice(cursor)
		for i := range cursor.IDs {
			zs[i].Altitude = s.relief.GroundAt(board.Center(bases[i].Pos)) + movers[i].Lift
		}
	}
}

var _ goke.System = (*shapingSystem)(nil)

// shapingSystem carries out the shaping commands — Raise, Lower, Level — as they come, in the
// interface part of the tick.
type shapingSystem struct {
	relief *Relief
	shape  *shaping
}

func (*shapingSystem) Init(*goke.SysInit) {}

func (s *shapingSystem) Update(*goke.CmdBuf, time.Duration) { s.shape.run(s.relief) }
