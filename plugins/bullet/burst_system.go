package bullet

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*burstSystem)(nil)

// burstSystem carries out the Bursts given: for the shot that gave itself one, every entity the
// space finds within the radius of its centre is a Blast for the rules, and the shot is gone.
type burstSystem struct {
	w      *world.Plugin
	space  *aabbworld.Space
	bursts *control.Queue[Burst]
	blasts *plugin.PairRules[Blast]
	tickOf plugin.TickSource

	seek *goke.Query
	base goke.Comp[world.Base]

	found []uid.UID64
}

func newBurstSystem(w *world.Plugin, bursts *control.Queue[Burst], blasts *plugin.PairRules[Blast]) *burstSystem {
	return &burstSystem{w: w, space: w.Space(), bursts: bursts, blasts: blasts, tickOf: w.Tick}
}

// sought is the one query the system offers its hosted rules.
const sought = 0

func (s *burstSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base)
	s.blasts.Bind(qb)
	s.seek = qb.Build()
}

func (s *burstSystem) Update(cb *goke.CmdBuf, dt time.Duration) {
	if s.bursts.Empty() {
		return
	}
	t := s.tickOf.Of(cb, dt)
	s.bursts.Drain(func(i control.Issued[Burst]) {
		if !i.ByEntity || !s.seek.Seek(i.Entity) {
			return
		}
		cur := s.seek.Cursor()
		self, centre, r := s.blasts.At(sought, cur), s.base.At(cur).Pos.Center(), i.Command.Radius
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
			s.blasts.Dispatch(t, self, s.blasts.At(sought, other), Blast{Self: i.Entity, Other: id, Distance: d})
		}
		s.w.Despawn(cb, i.Entity)
	})
}

// among reports whether id was found already.
func (s *burstSystem) among(id uid.UID64) bool {
	for _, f := range s.found {
		if f == id {
			return true
		}
	}
	return false
}
