package moments

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*standingSystem)(nil)

// takes unit.Entered off: whoever moves a unit into another cell puts it on after this pass
type standingSystem struct {
	r *Rules

	query *goke.Query
	base  goke.Comp[world.Base]
	at    goke.Comp[unit.At]
	mover goke.OptComp[unit.Mover]
	pace  goke.OptComp[steering.Pace]
	marks goke.OptComp[tag.Tags[unit.States]]
	z     goke.OptComp[world.Z]

	ids    []uid.UID64
	bases  []world.Base
	ats    []unit.At
	movers []unit.Mover
	zs     []world.Z
	states []tag.Tags[unit.States]
	about  func(i int) unit.Standing // bound once
}

func newStandingSystem(r *Rules) *standingSystem {
	s := &standingSystem{r: r}
	s.about = s.standing
	plugin.Own(&r.standing, &s.mover)
	plugin.Own(&r.standing, &s.pace)
	plugin.Own(&r.standing, &s.marks)
	plugin.Own(&r.standing, &s.z)
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
	for s.query.All(); s.query.Next(); {
		cursor := s.query.Cursor()
		s.ids, s.bases, s.ats, s.movers = cursor.IDs, s.base.Slice(cursor), s.at.Slice(cursor), s.mover.Slice(cursor)
		s.zs = s.z.Slice(cursor)
		s.states = s.marks.Slice(cursor)
		for i := range s.states {
			s.states[i] = s.states[i].Without(unit.Entered)
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

func (s *standingSystem) share(i int) float64 {
	b := &s.bases[i]
	at := b.Pos.Center()
	c, ok := s.r.grid.CellAt(at)
	if !ok {
		return 1
	}
	feet, reach := s.feet(i)
	kind, d := s.r.cells.KindFor(c, at, feet, reach, s.r.level), unit.DomainAt(s.movers, i)
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

func (s *standingSystem) logFall(i int) {
	st := s.standing(i)
	if !unit.Fallen().Holds(st) || s.r.told[st.ID] {
		return
	}
	if s.r.told == nil {
		s.r.told = map[uid.UID64]bool{}
	}
	s.r.told[st.ID] = true
	s.r.Log.Printf("entity %d fell into the %s at cell %d", st.ID, st.Kind.Name, st.Cell)
}

func (s *standingSystem) standing(i int) unit.Standing {
	at := s.bases[i].Pos.Center()
	c, ok := s.r.grid.CellAt(at)
	if !ok {
		c = s.ats[i].Cell
	}
	cells := s.r.cells
	feet, reach := s.feet(i)
	return unit.Standing{ID: s.ids[i], Cell: c, Kind: cells.KindFor(c, at, feet, reach, s.r.level),
		States: cells.States(c), Box: s.bases[i].Pos.AABB.AABB, Domain: unit.DomainAt(s.movers, i),
		Airborne: s.states != nil && s.states[i].Has(unit.Airborne)}
}

func (s *standingSystem) feet(i int) (float64, float64) {
	if s.zs == nil {
		return math.Inf(-1), 0
	}
	return s.zs[i].Altitude, s.zs[i].Reach()
}
