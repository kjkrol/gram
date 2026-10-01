package rule

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*standingSystem)(nil)

// standingSystem runs the rules of a unit.Standing over every entity carrying unit.At, after
// movement and collisions have had their say.
type standingSystem struct {
	r *Rules

	query *goke.Query
	base  goke.Comp[world.Base]
	at    goke.Comp[unit.At]
	mover goke.OptComp[unit.Mover]

	ids    []uid.UID64
	bases  []world.Base
	ats    []unit.At
	movers []unit.Mover
	about  func(i int) unit.Standing // standing, bound once
}

func newStandingSystem(r *Rules) *standingSystem {
	s := &standingSystem{r: r}
	s.about = s.standing
	host.Own(&r.standing, &s.mover)
	return s
}

func (s *standingSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.at)
	s.r.standing.Bind(qb)
	s.query = qb.Build()
}

func (s *standingSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	if s.r.standing.Empty() {
		return
	}
	tick := s.r.tick.Of(cb, d)
	tick.Around = s.r.around
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		s.ids, s.bases, s.ats, s.movers = cursor.IDs, s.base.Slice(cursor), s.at.Slice(cursor), s.mover.Slice(cursor)
		s.r.standing.Run(tick, cursor, s.about)
	}
}

// standing is where the i-th entity of the chunk being walked stands: the cell under its centre,
// or the one it is At off the board.
func (s *standingSystem) standing(i int) unit.Standing {
	c, ok := s.r.grid.CellAt(s.bases[i].Pos.Center())
	if !ok {
		c = s.ats[i].Cell
	}
	cells := s.r.cells
	return unit.Standing{ID: s.ids[i], Cell: c, Kind: cells.Kind(c), Places: cells.Tags(c), Box: s.bases[i].Pos.AABB.AABB, Domain: unit.DomainAt(s.movers, i)}
}
