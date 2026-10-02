package terrain

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*entitySystem)(nil)

// entitySystem gives every cell an entity at Setup, out of the seed, or finds the ones a save
// brought back, and hands them to the Cells. Every step it counts a change wherever an effect
// changed a cell — began or ended an Alter of its Ground (its effect.Changed on).
type entitySystem struct {
	cells *Cells

	active      *goke.Query // cells an effect was ever on
	activeComp  goke.Comp[effect.Active]
	activePlot  goke.Comp[cell.Plot]
	activeMarks goke.OptComp[tag.Tags[effect.States]]
	spawnPlot   goke.Comp[cell.Plot]
	spawnGround goke.Comp[cell.Ground]
	spawnWay    goke.Comp[cell.Way]
	spawnCross  goke.Comp[cell.Crossing]
	spawnMarks  goke.Comp[tag.Tags[effect.States]]
	spawnTags   goke.Comp[cell.Tags]
}

func (s *entitySystem) Init(si *goke.SysInit) {
	st := &store{ids: make([]uid.UID64, s.cells.grid.CellCount())}
	st.plots = si.NewQueryBuilder(&st.plot).Build()
	st.grounds = si.NewQueryBuilder(&st.ground).Build()
	st.ways = si.NewQueryBuilder(&st.way).Build()
	st.crossings = si.NewQueryBuilder(&st.crossing).Build()
	st.tagged = si.NewQueryBuilder(&st.plot).Optional(&st.tags).Build()
	s.active = si.NewQueryBuilder(&s.activeComp, &s.activePlot).Optional(&s.activeMarks).Build()

	found := 0
	for st.plots.All(); st.plots.Next(); {
		cur := st.plots.Cursor()
		plots := st.plot.Slice(cur)
		for i, id := range cur.IDs {
			if o, ok := s.cells.Ordinal(plots[i].Cell); ok {
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
	s.cells.made(st)
}

// spawn makes an entity for every cell out of the seed.
func (s *entitySystem) spawn(si *goke.SysInit, ids []uid.UID64) {
	seed := s.cells.seed
	cells := make([]cell.ID, len(ids))
	s.cells.grid.EachCell(func(c cell.ID) {
		if o, ok := s.cells.Ordinal(c); ok {
			cells[o] = c
		}
	})
	// the effects' markers and the game's tags of places, for good
	factory := si.NewFactory(&s.spawnPlot, &s.spawnGround, &s.spawnWay, &s.spawnCross, &s.spawnMarks, &s.spawnTags)
	factory.Create(len(cells))
	o := 0
	for factory.Next() {
		plots, grounds, ways := s.spawnPlot.Slice(&factory.Cursor), s.spawnGround.Slice(&factory.Cursor), s.spawnWay.Slice(&factory.Cursor)
		crossings, tags := s.spawnCross.Slice(&factory.Cursor), s.spawnTags.Slice(&factory.Cursor)
		for i, id := range factory.IDs {
			c := cells[o]
			plots[i] = cell.Plot{Cell: c}
			grounds[i] = cell.Ground{Kind: seed.Kind(c)}
			ways[i] = seed.Ways[c]
			crossings[i] = seed.Crossings[c]
			tags[i] = seed.Tags[c]
			ids[o] = id
			o++
		}
	}
}

func (s *entitySystem) Update(*goke.CmdBuf, time.Duration) {
	changed := false
	for s.active.All(); s.active.Next(); {
		cur := s.active.Cursor()
		plots, marks := s.activePlot.Slice(cur), s.activeMarks.Slice(cur)
		if marks == nil {
			continue
		}
		for i := range plots {
			if marks[i].Has(effect.Changed) {
				s.cells.touch(plots[i].Cell)
				changed = true
			}
		}
	}
	if changed {
		s.cells.version++
	}
}
