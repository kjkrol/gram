package moments

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
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
	pace  goke.OptComp[steering.Pace]

	ids    []uid.UID64
	bases  []world.Base
	ats    []unit.At
	movers []unit.Mover
	about  func(i int) unit.Standing // standing, bound once
}

func newStandingSystem(r *Rules) *standingSystem {
	s := &standingSystem{r: r}
	s.about = s.standing
	plugin.Own(&r.standing, &s.mover)
	plugin.Own(&r.standing, &s.pace)
	return s
}

func (s *standingSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.at)
	s.r.standing.Bind(qb)
	s.query = qb.Build()
}

func (s *standingSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	hosted := !s.r.standing.Empty()
	var tick plugin.Tick
	if hosted {
		tick = s.r.tick.Of(cb, d)
		tick.Around = s.r.around
	}
	tread := !s.r.now.Empty()
	if tread {
		if n := s.r.grid.CellCount(); len(s.r.trodden) != n {
			s.r.trodden = make([]bool, n)
		} else {
			clear(s.r.trodden)
		}
	}
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		s.ids, s.bases, s.ats, s.movers = cursor.IDs, s.base.Slice(cursor), s.at.Slice(cursor), s.mover.Slice(cursor)
		if tread {
			for i := range s.ids {
				s.tread(i)
			}
		}
		if paces := s.pace.Slice(cursor); paces != nil && s.movers != nil {
			for i := range paces {
				paces[i].Share = s.share(i)
			}
		}
		if s.r.Log != nil {
			for i := range s.ids {
				s.logFall(i)
			}
		}
		if hosted {
			s.r.standing.Run(tick, cursor, s.about)
		}
	}
}

// tread marks the cell under the i-th unit's centre as stood on this step.
func (s *standingSystem) tread(i int) {
	if c, ok := s.r.grid.CellAt(s.bases[i].Pos.Center()); ok {
		if o, ok := s.r.cells.Ordinal(c); ok && o < len(s.r.trodden) {
			s.r.trodden[o] = true
		}
	}
}

// share is the i-th unit's Pace: 1/CostFor(its domain) of the cell under its centre, and as the
// Map's slope says — slower up, quicker down — unless the kind is Graded.
func (s *standingSystem) share(i int) float64 {
	b := &s.bases[i]
	at := b.Pos.Center()
	c, ok := s.r.grid.CellAt(at)
	if !ok {
		return 1
	}
	kind, d := s.r.cells.Kind(c), unit.DomainAt(s.movers, i)
	share := 1.0
	if cost := kind.CostFor(d); cost > 0 {
		share /= cost
	}
	if kind.Graded || s.r.slope == nil {
		return share
	}
	dir := b.Vel.Dir
	if b.Vel.Value < 0 { // backing away: up or down the way it goes, not the way it faces
		dir = geom.NewVec(-dir.X, -dir.Y)
	}
	if sl := s.r.slope(at, dir, d); sl > 0 && sl != 1 {
		share /= sl
	}
	return share
}

// logFall writes a line, once for each, for the i-th unit fallen where its domain may not be.
func (s *standingSystem) logFall(i int) {
	st := s.standing(i)
	if !st.Fallen() || s.r.told[st.ID] {
		return
	}
	if s.r.told == nil {
		s.r.told = map[uid.UID64]bool{}
	}
	s.r.told[st.ID] = true
	s.r.Log.Printf("entity %d fell into the %s at cell %d", st.ID, st.Kind.Name, st.Cell)
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
