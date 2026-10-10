package terrain

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*builtSystem)(nil)

type builtSystem struct {
	cells   *Cells
	heights bool
	space   func() *aabbworld.Space

	query *goke.Query
	on    goke.Comp[cell.On]
	floor goke.OptComp[cell.Floor]
	wall  goke.OptComp[cell.Wall]

	now, last []site // by cell, one a cell: what is built now and what was the step before
}

type site struct {
	cell  cell.ID
	built cell.Built
}

// BuiltSystem gathers the Floors and Walls of the entities standing on the cells on the cells'
// entities every step, space the world's, where Standing finds them.
func (t *Cells) BuiltSystem(heights bool, space func() *aabbworld.Space) goke.System {
	return &builtSystem{cells: t, heights: heights, space: space}
}

func (s *builtSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.on).Optional(&s.floor).Optional(&s.wall).Build()
	st := s.cells.store
	for st.builts.All(); st.builts.Next(); {
		cur := st.builts.Cursor()
		plots, builts := st.plot.Slice(cur), st.built.Slice(cur)
		for i := range builts {
			if builts[i] != (cell.Built{}) {
				s.last = append(s.last, site{plots[i].Cell, builts[i]})
			}
		}
	}
	slices.SortFunc(s.last, bySite)
	s.cells.built = len(s.last)
	s.cells.standing = s
}

func (s *builtSystem) Update(*goke.CmdBuf, time.Duration) {
	s.now = s.now[:0]
	for s.query.All(); s.query.Next(); {
		cur := s.query.Cursor()
		ons, floors, walls := s.on.Slice(cur), s.floor.Slice(cur), s.wall.Slice(cur)
		if floors == nil && walls == nil {
			continue
		}
		for i := range ons {
			var b cell.Built
			if floors != nil {
				b.Floor = floors[i]
			}
			if walls != nil {
				b.Wall = walls[i]
			}
			if !s.heights && (b.Floor.Height != 0 || b.Wall.Height != 0) {
				panic(fmt.Sprintf("board: entity %d builds at a Height in a flat world; "+
					"set world.Config.Heights", cur.IDs[i]))
			}
			if b.Floor.Allows == 0 {
				b.Floor = cell.Floor{}
			}
			if b.Wall.Keeps == 0 {
				b.Wall = cell.Wall{}
			}
			if b != (cell.Built{}) {
				s.now = append(s.now, site{ons[i].Cell, b})
			}
		}
	}
	slices.SortFunc(s.now, bySite)
	s.now = gather(s.now)
	if slices.Equal(s.now, s.last) {
		return
	}
	changed := false
	set := func(c cell.ID, b cell.Built) {
		i, ok := s.cells.Ordinal(c)
		if !ok {
			return
		}
		if p := s.cells.store.builtOf(i); p != nil && *p != b {
			*p = b
			s.cells.touch(c)
			changed = true
		}
	}
	j := 0
	for _, l := range s.last {
		for j < len(s.now) && s.now[j].cell < l.cell {
			j++
		}
		if j == len(s.now) || s.now[j].cell != l.cell {
			set(l.cell, cell.Built{})
		}
	}
	for _, n := range s.now {
		set(n.cell, n.built)
	}
	if changed {
		s.cells.version++
	}
	s.cells.built = len(s.now)
	s.now, s.last = s.last, s.now
}

// standing asks the space at the cell's middle, where whatever stands on the cell lies.
func (s *builtSystem) standing(c cell.ID, each func(uid.UID64)) {
	space := s.space()
	if space == nil {
		return
	}
	middle := s.cells.grid.CellCenter(c)
	box := geom.NewAABBAt(geom.NewVec(middle.X-0.5, middle.Y-0.5), 1, 1)
	space.Query(box, aabbworld.AnyCapability, func(id uid.UID64) {
		if !s.query.SeekH(id) && !s.query.Seek(id) {
			return
		}
		if s.on.At(s.query.Cursor()).Cell == c {
			each(id)
		}
	})
}

func bySite(a, b site) int { return cmp.Compare(a.cell, b.cell) }

func gather(sites []site) []site {
	out := sites[:0]
	for _, s := range sites {
		if n := len(out); n > 0 && out[n-1].cell == s.cell {
			o := &out[n-1].built
			o.Floor.Allows |= s.built.Floor.Allows
			o.Floor.Height = max(o.Floor.Height, s.built.Floor.Height)
			if s.built.Wall.Keeps != 0 {
				o.Wall.Height = tallest(o.Wall, s.built.Wall)
				o.Wall.Keeps |= s.built.Wall.Keeps
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

// tallest is the higher Height of two walls, one of none standing at every height.
func tallest(a, b cell.Wall) float64 {
	switch {
	case a.Keeps == 0:
		return b.Height
	case a.Height == 0 || b.Height == 0:
		return 0
	}
	return max(a.Height, b.Height)
}
