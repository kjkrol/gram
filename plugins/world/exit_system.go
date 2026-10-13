package world

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/uid"
)

// Outside marks an entity wholly past an open edge, taken off once it is back inside.
type Outside struct{}

// Leaving is the moment of an entity carrying Outside, every tick it does. With no rule of it the
// world despawns the leaver.
type Leaving struct {
	ID   uid.UID64
	Base *Base
}

// Who is the entity leaving.
func (l Leaving) Who() uid.UID64 { return l.ID }

var _ goke.System = (*exitSystem)(nil)

type exitSystem struct {
	w    *module
	host *host.Rules[Leaving]

	query   *goke.Query
	base    goke.Comp[Base]
	outside goke.CompID

	// one gone already is not despawned again
	alive     *goke.Query
	aliveBase goke.Comp[Base]

	ids   []uid.UID64
	bases []Base
}

func newExitSystem(w *module, host *host.Rules[Leaving]) *exitSystem {
	return &exitSystem{w: w, host: host}
}

func (s *exitSystem) Init(si *goke.SysInit) {
	s.outside = si.RegComp[Outside]()
	qb := si.NewQueryBuilder(&s.base).Include(goke.Include[Outside]())
	s.host.Bind(qb)
	s.query = qb.Build()
	s.alive = si.NewQueryBuilder(&s.aliveBase).Build()
}

func (s *exitSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	s.w.despawns.Drain(func(i command.Issued[Despawn]) {
		if i.ByEntity && s.alive.Seek(i.Entity) {
			s.w.despawn(cb, i.Entity)
		}
	})
	tick := s.w.tick(cb, d)
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		s.ids, s.bases = cursor.IDs, s.base.Slice(cursor)
		if !s.host.Empty() {
			s.host.Run(tick, cursor, s.at)
		}
		for i, id := range s.ids {
			switch {
			case inside(s.w.space, s.bases[i].Pos.AABB.AABB):
				cb.RemoveCompOne(id, s.outside)
			case s.host.Empty():
				s.w.despawn(cb, id)
			}
		}
	}
}

func (s *exitSystem) at(i int) Leaving { return Leaving{ID: s.ids[i], Base: &s.bases[i]} }

func inside(space *aabbworld.Space, box geom.AABB) bool {
	width, height, edges := space.Bounds()
	outX := edges&aabbworld.OpenX != 0 && (box.BottomRight.X <= 0 || box.TopLeft.X >= float64(width))
	outY := edges&aabbworld.OpenY != 0 && (box.BottomRight.Y <= 0 || box.TopLeft.Y >= float64(height))
	return !outX && !outY
}

var (
	_ host.About = Leaving{}
)
