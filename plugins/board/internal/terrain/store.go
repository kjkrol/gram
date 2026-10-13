package terrain

import (
	"fmt"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

// a query a component, so a read seeks only the column it needs
type store struct {
	ids     []uid.UID64
	plots   *goke.Query
	grounds *goke.Query
	ways    *goke.Query
	builts  *goke.Query
	stated  *goke.Query
	plot    goke.Comp[cell.Plot]
	ground  goke.Comp[cell.Ground]
	way     goke.Comp[cell.Way]
	built   goke.Comp[cell.Built]
	states  goke.OptComp[tag.Tags[effect.States]]
}

func (s *store) groundOf(i int) *cell.Ground {
	s.seek(s.grounds, i)
	return s.ground.At(s.grounds.Cursor())
}

func (s *store) wayOf(i int) *cell.Way {
	s.seek(s.ways, i)
	return s.way.At(s.ways.Cursor())
}

// nil for a cell a save of an older build brought back without one
func (s *store) builtOf(i int) *cell.Built {
	if !s.builts.SeekH(s.ids[i]) && !s.builts.Seek(s.ids[i]) {
		return nil
	}
	return s.built.At(s.builts.Cursor())
}

func (s *store) statesOf(i int) tag.Tags[effect.States] {
	if !s.stated.SeekH(s.ids[i]) && !s.stated.Seek(s.ids[i]) {
		return 0
	}
	if t := s.states.At(s.stated.Cursor()); t != nil {
		return *t
	}
	return 0
}

func (s *store) setAll(kind cell.Kind) {
	for s.grounds.All(); s.grounds.Next(); {
		grounds := s.ground.Slice(s.grounds.Cursor())
		for i := range grounds {
			grounds[i].Kind = kind
		}
	}
}

func (s *store) seek(q *goke.Query, i int) {
	if !q.SeekH(s.ids[i]) && !q.Seek(s.ids[i]) {
		panic(fmt.Sprintf("board: the entity %d of a cell is gone", s.ids[i]))
	}
}
