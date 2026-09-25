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
// an effect rewrote a cell (effects.Active.Altered) or has just ended on one (effects.Idle).
type cellSystem struct {
	brd   *Board
	shape *shaping

	active      *goke.Query // cells under an effect
	idle        *goke.Query // cells whose last effect has just ended
	activeComp  goke.Comp[effects.Active]
	spawnPlot   goke.Comp[Plot]
	spawnGround goke.Comp[Ground]
}

func newCellSystem(brd *Board, shape *shaping) *cellSystem {
	return &cellSystem{brd: brd, shape: shape}
}

func (s *cellSystem) Init(si *goke.SysInit) {
	st := &cellStore{ids: make([]uid.UID64, s.brd.CellCount())}
	st.plots = si.NewQueryBuilder(&st.plot).Build()
	st.kinds = si.NewQueryBuilder(&st.ground).Build()
	s.active = si.NewQueryBuilder(&s.activeComp).Include(goke.Include[Plot]()).Build()
	s.idle = si.NewQueryBuilder().Include(goke.Include[Plot](), goke.Include[effects.Idle]()).Build()

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
	factory := si.NewFactory(&s.spawnPlot, &s.spawnGround)
	factory.Create(len(cells))
	o := 0
	for factory.Next() {
		plots, grounds := s.spawnPlot.Slice(&factory.Cursor), s.spawnGround.Slice(&factory.Cursor)
		for i, id := range factory.IDs {
			c := cells[o]
			plots[i] = Plot{Cell: c, Relief: s.brd.Relief(c)}
			grounds[i] = Ground{Kind: s.brd.Kind(c)}
			ids[o] = id
			o++
		}
	}
}

func (s *cellSystem) Update(*goke.CmdBuf, time.Duration) {
	s.shape.run(s.brd)
	changed := false
	for s.active.All(); s.active.Next() && !changed; {
		for _, a := range s.activeComp.Slice(s.active.Cursor()) {
			if a.Altered {
				changed = true
				break
			}
		}
	}
	for s.idle.All(); s.idle.Next() && !changed; {
		changed = len(s.idle.Cursor().IDs) > 0
	}
	if changed {
		s.brd.version++
	}
}
