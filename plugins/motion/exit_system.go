package motion

import (
	"time"

	cgoke "github.com/kjkrol/ceremony/goke"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*exitSystem)(nil)

type exitSystem struct {
	m    *module
	cols *cgoke.Columns

	query   *goke.Query
	base    goke.Comp[world.Base]
	outside goke.CompID
	pace    goke.Opt

	ids   []uid.UID64
	bases []world.Base
}

func newExitSystem(m *module) *exitSystem { return &exitSystem{m: m} }

func (s *exitSystem) Init(si *goke.SysInit) {
	s.outside = si.RegComp[Outside]()
	s.m.outside = s.outside
	s.m.set = true
	qb := si.NewQueryBuilder(&s.base).Include(goke.Include[Outside]())
	s.cols = cgoke.NewColumns(qb)
	s.m.leavers.Bind(s.cols)
	if s.pace != nil {
		qb.With(s.pace)
	}
	s.query = qb.Build()
}

func (s *exitSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	leavers := s.m.leavers
	w := s.m.w
	for s.query.Paced(d); s.query.Next(); {
		cursor, dt := s.query.Cursor(), s.query.Dt()
		s.ids, s.bases = cursor.IDs, s.base.Slice(cursor)
		if !leavers.Empty() {
			leavers.Run(w.Tick(dt), s.cols.Row(cursor), len(cursor.IDs), s.at)
		}
		for i, id := range s.ids {
			switch {
			case inside(w.Space(), s.bases[i].Pos.AABB.AABB):
				cb.RemoveCompOne(id, s.outside)
			case leavers.Empty():
				w.Despawn(cb, id)
			}
		}
	}
}

func (s *exitSystem) at(i int) Leaving { return Leaving{ID: s.ids[i], Base: &s.bases[i]} }

func inside(space *aabbworld.Space, box geom.AABB) bool {
	width, height, edges := space.Bounds()
	outX := edges&aabbworld.OpenX != 0 &&
		(box.BottomRight.X <= 0 || box.TopLeft.X >= float64(width))
	outY := edges&aabbworld.OpenY != 0 &&
		(box.BottomRight.Y <= 0 || box.TopLeft.Y >= float64(height))
	return !outX && !outY
}
