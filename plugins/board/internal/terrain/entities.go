package terrain

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/rule"
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
	spawnRoles  goke.Comp[tag.Tags[rule.Roles]]
	spawnWired  goke.Comp[rule.Wired]
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

// spawn makes an entity for every cell out of the seed: the cells wired to a wire apart, carrying
// their Wired besides.
func (s *entitySystem) spawn(si *goke.SysInit, ids []uid.UID64) {
	seed := s.cells.seed
	var plain, wired []int // ordinals
	cells := make([]cell.ID, len(ids))
	s.cells.grid.EachCell(func(c cell.ID) {
		if o, ok := s.cells.Ordinal(c); ok {
			cells[o] = c
		}
	})
	for o, c := range cells {
		if seed.Wired[c] != nil {
			wired = append(wired, o)
		} else {
			plain = append(plain, o)
		}
	}
	// the effects' markers, the game's tags of places and the roles, for good
	columns := []goke.Addable{&s.spawnPlot, &s.spawnGround, &s.spawnWay, &s.spawnCross, &s.spawnMarks, &s.spawnTags, &s.spawnRoles}
	s.make(si.NewFactory(columns...), plain, cells, ids, false)
	if len(wired) > 0 {
		s.make(si.NewFactory(append(columns, &s.spawnWired)...), wired, cells, ids, true)
	}
}

// make spawns the cells at ordinals with factory; wired, each with the Wired of its wire.
func (s *entitySystem) make(factory *goke.Factory, ordinals []int, cells []cell.ID, ids []uid.UID64, wired bool) {
	if len(ordinals) == 0 {
		return
	}
	seed := s.cells.seed
	factory.Create(len(ordinals))
	k := 0
	for factory.Next() {
		cur := &factory.Cursor
		plots, grounds, ways := s.spawnPlot.Slice(cur), s.spawnGround.Slice(cur), s.spawnWay.Slice(cur)
		crossings, tags, roles := s.spawnCross.Slice(cur), s.spawnTags.Slice(cur), s.spawnRoles.Slice(cur)
		var wires []rule.Wired
		if wired {
			wires = s.spawnWired.Slice(cur)
		}
		for i, id := range factory.IDs {
			o := ordinals[k]
			c := cells[o]
			plots[i] = cell.Plot{Cell: c}
			grounds[i] = cell.Ground{Kind: seed.Kind(c)}
			ways[i] = seed.Ways[c]
			crossings[i] = seed.Crossings[c]
			tags[i] = seed.Tags[c]
			roles[i] = seed.Roles[c]
			if wired {
				w := seed.Wired[c]
				to, made := w.Entity()
				if !made {
					panic(fmt.Sprintf("board: cell %d is wired to %v, which no world defined (world.Plugin.Wire)", c, w))
				}
				wires[i] = rule.Wired{To: to}
			}
			ids[o] = id
			k++
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
