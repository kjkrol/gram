package board

import (
	"fmt"
	"github.com/kjkrol/gram/plugins/board/cell"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/rule/effect"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*cellSystem)(nil)

// cellSystem gives every cell an entity at Setup, or finds the ones a save brought back, and hands
// them to the board. Every step it counts a change wherever an effect changed a cell's components —
// began or ended an Alter of its Ground (its effect.Changed on).
type cellSystem struct {
	brd *Board

	active      *goke.Query // cells an effect was ever on
	activeComp  goke.Comp[effect.Active]
	activePlot  goke.Comp[cell.Plot]
	activeMarks goke.OptComp[tag.Tags[effect.States]]
	spawnPlot   goke.Comp[cell.Plot]
	spawnGround goke.Comp[cell.Ground]
	spawnWay    goke.Comp[cell.Way]
	spawnCross  goke.Comp[cell.Crossing]
	spawnMarks  goke.Comp[tag.Tags[effect.States]]
	spawnPlaces goke.Comp[tag.Tags[cell.Family]]
}

func newCellSystem(brd *Board) *cellSystem { return &cellSystem{brd: brd} }

func (s *cellSystem) Init(si *goke.SysInit) {
	st := &cellStore{ids: make([]uid.UID64, s.brd.CellCount())}
	st.plots = si.NewQueryBuilder(&st.plot).Build()
	st.kinds = si.NewQueryBuilder(&st.ground).Build()
	st.ways = si.NewQueryBuilder(&st.way).Build()
	st.crossings = si.NewQueryBuilder(&st.crossing).Build()
	st.tagged = si.NewQueryBuilder(&st.plot).Optional(&st.places).Build()
	s.active = si.NewQueryBuilder(&s.activeComp, &s.activePlot).Optional(&s.activeMarks).Build()

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
	cells := make([]cell.ID, len(ids))
	s.brd.EachCell(func(c cell.ID) {
		if o, ok := s.brd.Ordinal(c); ok {
			cells[o] = c
		}
	})
	// the effects' markers and the game's tags of places, for good
	factory := si.NewFactory(&s.spawnPlot, &s.spawnGround, &s.spawnWay, &s.spawnCross, &s.spawnMarks, &s.spawnPlaces)
	factory.Create(len(cells))
	o := 0
	for factory.Next() {
		plots, grounds, ways := s.spawnPlot.Slice(&factory.Cursor), s.spawnGround.Slice(&factory.Cursor), s.spawnWay.Slice(&factory.Cursor)
		crossings, places := s.spawnCross.Slice(&factory.Cursor), s.spawnPlaces.Slice(&factory.Cursor)
		for i, id := range factory.IDs {
			c := cells[o]
			plots[i] = cell.Plot{Cell: c}
			places[i] = s.brd.places[c]
			grounds[i] = cell.Ground{Kind: s.brd.seed.Kind(c)}
			ways[i] = s.brd.Way(c)
			crossings[i] = s.brd.Crossing(c)
			ids[o] = id
			o++
		}
	}
	s.brd.places = nil
}

func (s *cellSystem) Update(*goke.CmdBuf, time.Duration) {
	changed := false
	for s.active.All(); s.active.Next(); {
		cur := s.active.Cursor()
		plots, marks := s.activePlot.Slice(cur), s.activeMarks.Slice(cur)
		if marks == nil {
			continue
		}
		for i := range plots {
			if marks[i].Has(effect.Changed) {
				s.brd.touch(plots[i].Cell)
				changed = true
			}
		}
	}
	if changed {
		s.brd.version++
	}
}
