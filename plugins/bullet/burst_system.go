package bullet

import (
	cgoke "github.com/kjkrol/ceremony/goke"
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*burstSystem)(nil)

type burstSystem struct {
	w      *world.Plugin
	space  *aabbworld.Space
	bursts *command.Queue[Burst]
	blasts *host.PairRules[Blast]
	cols   *cgoke.Columns
	tickOf host.TickSource

	seek *goke.Query
	base goke.Comp[world.Base]

	found []uid.UID64
}

func newBurstSystem(w *world.Plugin, bursts *command.Queue[Burst], blasts *host.PairRules[Blast]) *burstSystem {
	return &burstSystem{w: w, space: w.Write().Space(), bursts: bursts, blasts: blasts,
		tickOf: w.Read(0).Tick}
}

const sought = 0

func (s *burstSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base)
	s.cols = cgoke.NewColumns(qb)
	s.blasts.Bind(s.cols)
	s.seek = qb.Build()
}

func (s *burstSystem) Update(cb *goke.CmdBuf, dt time.Duration) {
	if s.bursts.Empty() {
		return
	}
	t := s.tickOf.Of(dt)
	s.bursts.Drain(func(i command.Issued[Burst]) {
		if !i.ByEntity || !s.seek.Seek(i.Entity) {
			return
		}
		cur := s.seek.Cursor()
		self, centre, r := s.blasts.Read(sought, s.cols.Row(cur), int(cur.Slot)), s.base.At(cur).Pos.Center(), i.Command.Radius
		s.found = s.found[:0]
		s.space.Query(geom.NewAABB(geom.NewVec(centre.X-r, centre.Y-r), geom.NewVec(centre.X+r, centre.Y+r)), aabbworld.AnyCapability, func(id uid.UID64) {
			if id != i.Entity && !s.among(id) {
				s.found = append(s.found, id)
			}
		})
		for _, id := range s.found {
			if !s.seek.Seek(id) {
				continue
			}
			other := s.seek.Cursor()
			at := s.base.At(other).Pos.Center()
			d := math.Hypot(at.X-centre.X, at.Y-centre.Y)
			if d > r {
				continue
			}
			s.blasts.Dispatch(t, self, s.blasts.Read(sought, s.cols.Row(other), int(other.Slot)), Blast{Self: i.Entity, Other: id, Distance: d})
		}
		s.w.Write().Despawn(cb, i.Entity)
	})
}

func (s *burstSystem) among(id uid.UID64) bool {
	for _, f := range s.found {
		if f == id {
			return true
		}
	}
	return false
}
