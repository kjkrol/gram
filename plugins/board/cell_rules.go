package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*cellRuleSystem)(nil)

// cellRuleSystem runs the rules of a Cell over every cell, after the standing report.
type cellRuleSystem struct {
	brd    *Board
	host   *host.EachHost[cell.Now]
	tick   plugin.TickSource // the world's
	around func(moment any, rings int, each func(uid.UID64))

	query  *goke.Query
	plot   goke.Comp[cell.Plot]
	ground goke.Comp[cell.Ground]

	ids     []uid.UID64
	plots   []cell.Plot
	grounds []cell.Ground
	about   func(i int) cell.Now // at, bound once
}

func newCellRuleSystem(brd *Board, each *host.EachHost[cell.Now]) *cellRuleSystem {
	s := &cellRuleSystem{brd: brd, host: each, around: brd.placesAround}
	s.about = s.at
	return s
}

func (s *cellRuleSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.plot, &s.ground)
	s.host.Bind(qb)
	s.query = qb.Build()
}

func (s *cellRuleSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	if s.host.Empty() {
		return
	}
	tick := s.tick.Of(cb, d)
	tick.Around = s.around
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		s.ids, s.plots, s.grounds = cursor.IDs, s.plot.Slice(cursor), s.ground.Slice(cursor)
		s.host.Run(tick, cursor, s.about)
	}
}

// at describes the i-th cell of the chunk being walked.
func (s *cellRuleSystem) at(i int) cell.Now {
	return cell.Now{ID: s.ids[i], Cell: s.plots[i].Cell, Kind: s.grounds[i].Kind}
}
