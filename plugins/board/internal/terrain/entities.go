package terrain

import (
	"fmt"
	"github.com/kjkrol/ceremony"
	cgoke "github.com/kjkrol/ceremony/goke"
	"reflect"
	"time"

	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*entitySystem)(nil)

type entitySystem struct {
	cells     *Cells
	roster    *kind.Roster
	template  *kind.Template
	extra     *cgoke.Spawner
	mattering func() tag.Tags[effect.States] // the effects the terrain changes under
	matters   tag.Tags[effect.States]
	known     bool
	was       []tag.Tags[effect.States] // a cell's effects as it last changed, by ordinal

	active      *goke.Query
	activeComp  goke.Comp[cgoke.Active]
	activePlot  goke.Comp[cell.Plot]
	activeMarks goke.OptComp[tag.Tags[effect.States]]
	spawnPlot   goke.Comp[cell.Plot]
	spawnGround goke.Comp[cell.Ground]
	spawnWay    goke.Comp[cell.Way]
	spawnBuilt  goke.Comp[cell.Built]
	spawnMarks  goke.Comp[tag.Tags[effect.States]]
	spawnRoles  goke.Comp[tag.Tags[cond.Roles]]
	spawnLabel  goke.Comp[ceremony.Label]
}

func (s *entitySystem) Init(si *goke.SysInit) {
	st := &store{ids: make([]uid.UID64, s.cells.grid.CellCount())}
	st.plots = si.NewQueryBuilder(&st.plot).Build()
	st.grounds = si.NewQueryBuilder(&st.ground).Build()
	st.ways = si.NewQueryBuilder(&st.way).Build()
	st.builts = si.NewQueryBuilder(&st.plot, &st.built).Build()
	st.stated = si.NewQueryBuilder(&st.plot).Optional(&st.states).Build()
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
	s.cells.fresh = found == 0
	switch found {
	case 0:
		s.spawn(si, st.ids)
	case len(st.ids):
	default:
		panic(fmt.Sprintf("board: the save holds %d cells, the board has %d", found, len(st.ids)))
	}
	s.cells.made(st)
}

// the cells called something are made apart: few, so whoever looks for a name walks few
func (s *entitySystem) spawn(si *goke.SysInit, ids []uid.UID64) {
	seed := s.cells.seed
	var plain, labelled []int // ordinals
	cells := make([]cell.ID, len(ids))
	s.cells.grid.EachCell(func(c cell.ID) {
		if o, ok := s.cells.Ordinal(c); ok {
			cells[o] = c
		}
	})
	for o, c := range cells {
		if seed.Labels[c] != (ceremony.Label{}) {
			labelled = append(labelled, o)
		} else {
			plain = append(plain, o)
		}
	}
	columns := []goke.Addable{
		&s.spawnPlot,
		&s.spawnGround,
		&s.spawnWay,
		&s.spawnBuilt,
		&s.spawnMarks,
		&s.spawnRoles,
	}
	s.extra = cgoke.NewSpawner(s.templated()...)
	columns = append(columns, s.extra.Columns()...)
	s.make(si.NewFactory(columns...), plain, cells, ids, false)
	if len(labelled) > 0 {
		s.make(si.NewFactory(append(columns, &s.spawnLabel)...), labelled, cells, ids, true)
	}
}

func (s *entitySystem) make(factory *goke.Factory, ordinals []int, cells []cell.ID, ids []uid.UID64, labelled bool) {
	if len(ordinals) == 0 {
		return
	}
	seed := s.cells.seed
	factory.Create(len(ordinals))
	k := 0
	for factory.Next() {
		cur := &factory.Cursor
		plots, grounds, ways := s.spawnPlot.Slice(cur), s.spawnGround.Slice(cur), s.spawnWay.Slice(cur)
		roles := s.spawnRoles.Slice(cur)
		var labels []ceremony.Label
		if labelled {
			labels = s.spawnLabel.Slice(cur)
		}
		for i, id := range factory.IDs {
			o := ordinals[k]
			c := cells[o]
			plots[i] = cell.Plot{Cell: c}
			grounds[i] = cell.Ground{Kind: seed.Kind(c)}
			ways[i] = seed.Ways[c]
			roles[i] = seed.Roles[c]
			if labelled {
				labels[i] = seed.Labels[c]
			}
			s.extra.Write(cur, i, c)
			ids[o] = id
			k++
		}
	}
}

var own = map[reflect.Type]bool{
	reflect.TypeFor[cell.Plot]():               true,
	reflect.TypeFor[cell.Ground]():             true,
	reflect.TypeFor[cell.Way]():                true,
	reflect.TypeFor[cell.Built]():              true,
	reflect.TypeFor[tag.Tags[effect.States]](): true,
	reflect.TypeFor[tag.Tags[cond.Roles]]():    true,
	reflect.TypeFor[ceremony.Label]():          true,
}

func (s *entitySystem) templated() kind.Spec {
	if s.template == nil {
		return nil
	}
	spec := s.roster.Spec(s.template)
	for _, c := range spec {
		if own[c.Type()] {
			panic(fmt.Sprintf("board: the cells' template gives %v, which the board gives every cell itself", c.Type()))
		}
		if row := c.Row(); row != nil && row != reflect.TypeFor[cell.ID]() {
			panic(fmt.Sprintf("board: the cells' template loads %v from a %v; a cell's row is its cell.ID", c.Type(), row))
		}
	}
	return spec
}

func (s *entitySystem) Update(*goke.CmdBuf, time.Duration) {
	if !s.known {
		s.matters, s.known = ^tag.Tags[effect.States](0), true
		if s.mattering != nil {
			s.matters = s.mattering()
		}
		s.was = make([]tag.Tags[effect.States], s.cells.grid.CellCount())
	}
	changed := false
	for s.active.All(); s.active.Next(); {
		cur := s.active.Cursor()
		plots, marks := s.activePlot.Slice(cur), s.activeMarks.Slice(cur)
		if marks == nil {
			continue
		}
		for i := range plots {
			if !marks[i].Has(effect.Changed) {
				continue
			}
			// the terrain changes as an effect it changes under begins or ends; one turning only
			// what it does not hold leaves it as it is
			now := marks[i].Without(effect.Changed)
			o, ok := s.cells.Ordinal(plots[i].Cell)
			if ok && o < len(s.was) {
				was := s.was[o]
				s.was[o] = now
				if (now^was)&s.matters == 0 {
					continue
				}
			}
			s.cells.touch(plots[i].Cell)
			changed = true
		}
	}
	if changed {
		s.cells.version++
	}
}
