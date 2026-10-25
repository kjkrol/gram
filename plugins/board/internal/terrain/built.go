package terrain

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*builtSystem)(nil)

type builtSystem struct {
	cells *Cells
	space func() *aabbworld.Space

	query *goke.Query
	on    goke.Comp[cell.On]
	floor goke.OptComp[cell.Floor]
	wall  goke.OptComp[cell.Wall]
	rail  goke.OptComp[cell.Rail]
	base  goke.OptComp[entity.Base]
	lie   goke.OptComp[entity.Lie]

	now, last []site
	raw       []site // as the entities have it, before the rules
	checked   uint64
	laid      bool // after the first gathering a broken bridge no longer panics
}

type site struct {
	cell  cell.ID
	built cell.Built
}

// BuiltSystem gathers what the entities standing on the cells build onto the cells' entities
// every step; space is the world's, where Standing finds them.
func (t *Cells) BuiltSystem(space func() *aabbworld.Space) goke.System {
	return &builtSystem{cells: t, space: space}
}

func (s *builtSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.on).
		Optional(&s.floor).Optional(&s.wall).Optional(&s.rail).
		Optional(&s.base).Optional(&s.lie).
		Build()
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
	s.cells.built, s.cells.railed = len(s.last), railed(s.last)
	s.cells.decked(s.last)
	s.cells.standing = s
}

func (s *builtSystem) Update(*goke.CmdBuf, time.Duration) {
	s.now = s.now[:0]
	for s.query.All(); s.query.Next(); {
		cur := s.query.Cursor()
		ons, floors, walls := s.on.Slice(cur), s.floor.Slice(cur), s.wall.Slice(cur)
		rails := s.rail.Slice(cur)
		if floors == nil && walls == nil && rails == nil {
			continue
		}
		s.lay(floors, rails, s.base.Slice(cur), s.lie.Slice(cur))
		for i := range ons {
			var b cell.Built
			if floors != nil {
				b.Floor = floors[i]
			}
			if walls != nil {
				b.Wall = walls[i]
			}
			if rails != nil {
				b.Rail = rails[i]
			}
			s.refuse(cur.IDs[i], b.Floor)
			if b.Floor.Allows == 0 {
				b.Floor = cell.Floor{}
			}
			if b.Wall.Keeps == 0 {
				b.Wall = cell.Wall{}
			}
			if b.Rail.Keeps == 0 {
				b.Rail = cell.Rail{}
			}
			if b != (cell.Built{}) {
				s.now = append(s.now, site{ons[i].Cell, b})
			}
		}
	}
	if !slices.IsSortedFunc(s.now, bySite) {
		slices.SortFunc(s.now, bySite)
	}
	s.now = gather(s.now)
	for k := range s.now {
		b := &s.now[k].built
		if b.Rail.Keeps == 0 {
			continue
		}
		if _, through, _ := s.cells.run(b.Floor); !through || !banded(b) {
			b.Rail = cell.Rail{} // a railing stands along a band carrying someone through alone
		}
	}
	if s.laid && s.cells.version == s.checked && slices.Equal(s.now, s.raw) {
		return
	}
	s.raw = append(s.raw[:0], s.now...)
	s.cells.keepBridges(s.now, !s.laid && s.cells.fresh)
	s.laid = true
	if slices.Equal(s.now, s.last) {
		s.checked = s.cells.version
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
	s.cells.built, s.cells.railed = len(s.now), railed(s.now)
	s.cells.decked(s.now)
	s.now, s.last = s.last, s.now
	s.checked = s.cells.version
}

func (s *builtSystem) lay(floors []cell.Floor, rails []cell.Rail, bases []entity.Base,
	lies []entity.Lie) {
	if floors == nil || lies == nil {
		return
	}
	for i := range floors {
		f := &floors[i]
		if f.Width <= 0 || f.Links == 0 {
			lies[i] = entity.Lie{}
			continue
		}
		d, through, ok := s.cells.run(*f)
		if !ok {
			lies[i] = entity.Lie{}
			continue
		}
		step := s.cells.grid.Step(d)
		l := f32.Hypot(step.X, step.Y)
		lie := entity.Lie{Length: l, Width: f.Width}
		switch {
		case !through: // a head: from the middle into the bridge
			lie.Length, lie.Ahead = l/2, l/4
		case rails != nil && rails[i].Keeps != 0 && f.Allows != 0:
			lie.Edged, lie.Edge = true, rails[i].Height
		}
		lies[i] = lie
		if bases != nil {
			bases[i].Vel.Dir = geom.NewVec(step.X/l, step.Y/l)
		}
	}
}

func (s *builtSystem) refuse(id uid.UID64, f cell.Floor) {
	switch {
	case f.Width < 0:
		panic(fmt.Sprintf("board: entity %d's floor is %v wide", id, f.Width))
	case f.Width > 0 && f.Links == 0:
		panic(fmt.Sprintf("board: entity %d's floor is a band running nowhere: give it Links", id))
	case f.Width > 0 && f.Width > s.widest(f.Links):
		panic(fmt.Sprintf("board: entity %d's floor is %v wide, its cell lets it be %v", id,
			f.Width, s.widest(f.Links)))
	}
}

func (s *builtSystem) widest(links cell.Links) float32 {
	g := s.cells.grid
	most := f32.Inf(1)
	for i := range g.Directions() {
		if !links.Has(i) {
			continue
		}
		if !g.Corner(i) {
			return g.CellSpan()
		}
		st := g.Step(i)
		most = min(most, f32.Hypot(st.X, st.Y))
	}
	return most
}

func railed(sites []site) int {
	n := 0
	for _, s := range sites {
		if s.built.Rail.Keeps != 0 {
			n++
		}
	}
	return n
}

// whatever stands on a cell lies at its middle
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
			o.Floor.Width = widest(o.Floor, s.built.Floor)
			o.Floor.Allows |= s.built.Floor.Allows
			o.Floor.Height = max(o.Floor.Height, s.built.Floor.Height)
			o.Floor.Graded = o.Floor.Graded || s.built.Floor.Graded
			o.Floor.Thickness = max(o.Floor.Thickness, s.built.Floor.Thickness)
			if c := s.built.Floor.Cost; c > 0 && (o.Floor.Cost == 0 || c < o.Floor.Cost) {
				o.Floor.Cost = c
			}
			if s.built.Wall.Keeps != 0 {
				o.Wall.Height = tallest(o.Wall, s.built.Wall)
				o.Wall.Keeps |= s.built.Wall.Keeps
			}
			o.Floor.Links |= s.built.Floor.Links
			o.Rail.Keeps |= s.built.Rail.Keeps
			o.Rail.Height = max(o.Rail.Height, s.built.Rail.Height)
			continue
		}
		out = append(out, s)
	}
	return out
}

func widest(a, b cell.Floor) float32 {
	switch {
	case b.Allows == 0:
		return a.Width
	case a.Allows == 0:
		return b.Width
	case a.Width == 0 || b.Width == 0:
		return 0
	}
	return max(a.Width, b.Width)
}

// a Height of 0 stands at every height
func tallest(a, b cell.Wall) float32 {
	switch {
	case a.Keeps == 0:
		return b.Height
	case a.Height == 0 || b.Height == 0:
		return 0
	}
	return max(a.Height, b.Height)
}
