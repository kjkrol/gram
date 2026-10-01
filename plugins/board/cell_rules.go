package board

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/uid"
)

// Cell is a cell of the board at a step, as the rules the board hosts get it: the cell's entity,
// which cell it is, and its kind now, effects on its ground included. Board hosts rules of it,
// run for every cell every step while one is hooked; they filter cells by the game's tags of
// places (rule.Self(trapdoor)) and by effects' markers (rule.Self(burning.Mark())).
type Cell struct {
	ID   uid.UID64 // the cell's entity
	Cell CellID    // which cell
	Kind CellKind  // its kind now, effects on its ground included
}

// Who is the cell's entity: whose moment it is, for a rule.
func (c Cell) Who() uid.UID64 { return c.ID }

// Placed: a rule's Here acts on this cell, its Around on the rings round it too.
func (Cell) Placed() {}

var _ goke.System = (*cellRuleSystem)(nil)

// cellRuleSystem runs the rules of a Cell over every cell, after the standing report.
type cellRuleSystem struct {
	brd    *Board
	host   *host.EachHost[Cell]
	tick   plugin.TickSource // the world's
	around func(moment any, rings int, each func(uid.UID64))

	query  *goke.Query
	plot   goke.Comp[Plot]
	ground goke.Comp[Ground]

	ids     []uid.UID64
	plots   []Plot
	grounds []Ground
	about   func(i int) Cell // at, bound once
}

func newCellRuleSystem(brd *Board, each *host.EachHost[Cell]) *cellRuleSystem {
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
func (s *cellRuleSystem) at(i int) Cell {
	return Cell{ID: s.ids[i], Cell: s.plots[i].Cell, Kind: s.grounds[i].Kind}
}
