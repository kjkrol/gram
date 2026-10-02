package moments

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*cellSystem)(nil)

// cellSystem runs the rules of a cell.Now over every cell, after the standing pass.
type cellSystem struct {
	r *Rules

	query  *goke.Query
	plot   goke.Comp[cell.Plot]
	ground goke.Comp[cell.Ground]

	ids     []uid.UID64
	plots   []cell.Plot
	grounds []cell.Ground
	about   func(i int) cell.Now // now, bound once
}

func newCellSystem(r *Rules) *cellSystem {
	s := &cellSystem{r: r}
	s.about = s.now
	return s
}

func (s *cellSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.plot, &s.ground)
	s.r.now.Bind(qb)
	s.query = qb.Build()
}

func (s *cellSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	if s.r.now.Empty() {
		return
	}
	tick := s.r.tick.Of(cb, d)
	tick.Around = s.r.around
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		s.ids, s.plots, s.grounds = cursor.IDs, s.plot.Slice(cursor), s.ground.Slice(cursor)
		s.r.now.Run(tick, cursor, s.about)
	}
}

// now is the i-th cell of the chunk being walked.
func (s *cellSystem) now(i int) cell.Now {
	c := s.plots[i].Cell
	trodden := false
	if o, ok := s.r.cells.Ordinal(c); ok && o < len(s.r.trodden) {
		trodden = s.r.trodden[o]
	}
	return cell.Now{ID: s.ids[i], Cell: c, Kind: s.grounds[i].Kind, Trodden: trodden}
}
