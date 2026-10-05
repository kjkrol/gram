package terrain

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// store is where the cells' entities are, by their ordinals: a query for each of their components,
// so a read seeks only the column it needs.
type store struct {
	ids       []uid.UID64
	plots     *goke.Query
	grounds   *goke.Query
	ways      *goke.Query
	crossings *goke.Query
	tagged    *goke.Query
	stated    *goke.Query
	plot      goke.Comp[cell.Plot]
	ground    goke.Comp[cell.Ground]
	way       goke.Comp[cell.Way]
	crossing  goke.Comp[cell.Crossing]
	tags      goke.OptComp[cell.Tags]
	states    goke.OptComp[tag.Tags[effect.States]]
}

// groundOf is the i-th cell's Ground, in place.
func (s *store) groundOf(i int) *cell.Ground {
	s.seek(s.grounds, i)
	return s.ground.At(s.grounds.Cursor())
}

// wayOf is the i-th cell's Way, in place.
func (s *store) wayOf(i int) *cell.Way {
	s.seek(s.ways, i)
	return s.way.At(s.ways.Cursor())
}

// crossingOf is the i-th cell's Crossing, in place.
func (s *store) crossingOf(i int) *cell.Crossing {
	s.seek(s.crossings, i)
	return s.crossing.At(s.crossings.Cursor())
}

// tagsOf are the game's tags of places the i-th cell carries.
func (s *store) tagsOf(i int) cell.Tags {
	if !s.tagged.SeekH(s.ids[i]) && !s.tagged.Seek(s.ids[i]) {
		return 0
	}
	if t := s.tags.At(s.tagged.Cursor()); t != nil {
		return *t
	}
	return 0
}

// statesOf are the markers of the effects on the i-th cell.
func (s *store) statesOf(i int) tag.Tags[effect.States] {
	if !s.stated.SeekH(s.ids[i]) && !s.stated.Seek(s.ids[i]) {
		return 0
	}
	if t := s.states.At(s.stated.Cursor()); t != nil {
		return *t
	}
	return 0
}

// setAll sets every cell's Ground to kind.
func (s *store) setAll(kind cell.Kind) {
	for s.grounds.All(); s.grounds.Next(); {
		grounds := s.ground.Slice(s.grounds.Cursor())
		for i := range grounds {
			grounds[i].Kind = kind
		}
	}
}

// seek puts q's cursor on the i-th cell's entity, which lives as long as the board.
func (s *store) seek(q *goke.Query, i int) {
	if !q.SeekH(s.ids[i]) && !q.Seek(s.ids[i]) {
		panic(fmt.Sprintf("board: the entity %d of a cell is gone", s.ids[i]))
	}
}
