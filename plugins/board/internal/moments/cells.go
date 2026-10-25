package moments

import (
	"time"

	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*cellSystem)(nil)

type cellSystem struct {
	cols *cgoke.Columns
	r    *Rules

	query  *goke.Query
	plot   goke.Comp[cell.Plot]
	ground goke.Comp[cell.Ground]

	ids     []uid.UID64
	plots   []cell.Plot
	grounds []cell.Ground
	about   func(i int) cell.Now // bound once
}

func newCellSystem(r *Rules) *cellSystem {
	s := &cellSystem{r: r}
	s.about = s.now
	return s
}

func (s *cellSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.plot, &s.ground)
	s.cols = cgoke.NewColumns(qb)
	s.r.now.Bind(s.cols)
	s.query = qb.Build()
}

func (s *cellSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	if s.r.now.Empty() {
		return
	}
	tick := s.r.tick.Of(d)
	tick.Places = s.r.places
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		s.ids, s.plots, s.grounds = cursor.IDs, s.plot.Slice(cursor), s.ground.Slice(cursor)
		s.r.now.Run(tick, s.cols.Row(cursor), len(cursor.IDs), s.about)
	}
}

func (s *cellSystem) now(i int) cell.Now {
	return cell.Now{ID: s.ids[i], Cell: s.plots[i].Cell, Kind: s.grounds[i].Kind}
}
