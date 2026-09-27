package board

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/effects"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*cellSystem)(nil)

// cellSystem gives every cell an entity at Setup, or finds the ones a save brought back, and hands
// them to the board. Every tick it carries out the shaping commands and counts a change wherever
// an effect rewrote a cell (effects.Active.Altered) or has just ended on one (effects.Idle),
// sealing its corners to its neighbours'.
type cellSystem struct {
	brd   *Board
	shape *shaping

	active      *goke.Query // cells under an effect
	idle        *goke.Query // cells whose last effect has just ended
	activeComp  goke.Comp[effects.Active]
	activePlot  goke.Comp[Plot]
	idlePlot    goke.Comp[Plot]
	spawnPlot   goke.Comp[Plot]
	spawnGround goke.Comp[Ground]
	spawnWay    goke.Comp[Way]
	spawnCross  goke.Comp[Crossing]
}

func newCellSystem(brd *Board, shape *shaping) *cellSystem {
	return &cellSystem{brd: brd, shape: shape}
}

func (s *cellSystem) Init(si *goke.SysInit) {
	st := &cellStore{ids: make([]uid.UID64, s.brd.CellCount())}
	st.plots = si.NewQueryBuilder(&st.plot).Build()
	st.kinds = si.NewQueryBuilder(&st.ground).Build()
	st.ways = si.NewQueryBuilder(&st.way).Build()
	st.crossings = si.NewQueryBuilder(&st.crossing).Build()
	s.active = si.NewQueryBuilder(&s.activeComp, &s.activePlot).Build()
	s.idle = si.NewQueryBuilder(&s.idlePlot).Include(goke.Include[effects.Idle]()).Build()

	found := 0
	for st.plots.All(); st.plots.Next(); {
		cur := st.plots.Cursor()
		plots := st.plot.Slice(cur)
		for i, id := range cur.IDs {
			if o, ok := s.brd.Ordinal(plots[i].Cell); ok {
				st.ids[o] = id
				found++
			}
		}
	}
	switch found {
	case 0:
		s.spawn(si, st.ids)
	case len(st.ids):
	default:
		panic(fmt.Sprintf("board: the save holds %d cells, the board has %d", found, len(st.ids)))
	}
	s.brd.bind(st)
}

// spawn makes an entity for every cell out of the board's seed.
func (s *cellSystem) spawn(si *goke.SysInit, ids []uid.UID64) {
	cells := make([]CellID, len(ids))
	s.brd.EachCell(func(c CellID) {
		if o, ok := s.brd.Ordinal(c); ok {
			cells[o] = c
		}
	})
	factory := si.NewFactory(&s.spawnPlot, &s.spawnGround, &s.spawnWay, &s.spawnCross)
	factory.Create(len(cells))
	o := 0
	for factory.Next() {
		plots, grounds, ways := s.spawnPlot.Slice(&factory.Cursor), s.spawnGround.Slice(&factory.Cursor), s.spawnWay.Slice(&factory.Cursor)
		crossings := s.spawnCross.Slice(&factory.Cursor)
		for i, id := range factory.IDs {
			c := cells[o]
			plots[i] = Plot{Cell: c, Relief: s.brd.Relief(c)}
			grounds[i] = Ground{Kind: s.brd.seed.Kind(c)}
			ways[i] = s.brd.Way(c)
			crossings[i] = s.brd.Crossing(c)
			ids[o] = id
			o++
		}
	}
}

func (s *cellSystem) Update(*goke.CmdBuf, time.Duration) {
	s.shape.run(s.brd)
	changed := false
	for s.active.All(); s.active.Next(); {
		cur := s.active.Cursor()
		plots := s.activePlot.Slice(cur)
		for i, a := range s.activeComp.Slice(cur) {
			if a.Altered {
				s.brd.touch(plots[i].Cell)
				s.brd.seal(plots[i].Cell)
				changed = true
			}
		}
	}
	for s.idle.All(); s.idle.Next(); {
		for _, p := range s.idlePlot.Slice(s.idle.Cursor()) {
			s.brd.touch(p.Cell)
			s.brd.seal(p.Cell)
			changed = true
		}
	}
	if changed {
		s.brd.version++
	}
}
