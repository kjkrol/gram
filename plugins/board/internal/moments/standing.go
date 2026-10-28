package moments

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
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
	cols   *cgoke.Columns
	dirs   []geom.Vec // the grid's directions, each of length 1
}

func newStandingSystem(r *Rules) *standingSystem {
	s := &standingSystem{r: r}
	s.about = s.standing
	for i := range r.grid.Directions() {
		step := r.grid.Step(i)
		length := f32.Hypot(step.X, step.Y)
		s.dirs = append(s.dirs, geom.NewVec(step.X/length, step.Y/length))
	}
	return s
}

// the direction nearest the way v points, as grid.Nearest finds it, over the unit vectors kept
func (s *standingSystem) nearest(v geom.Vec) int {
	best, most := 0, f32.Inf(-1)
	for i, d := range s.dirs {
		if dot := d.X*v.X + d.Y*v.Y; dot > most {
			best, most = i, dot
		}
	}
	return best
}

func (s *standingSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.at)
	s.cols = cgoke.NewColumns(qb)
	s.cols.Own(&s.mover)
	s.cols.Own(&s.pace)
	s.cols.Own(&s.marks)
	s.cols.Own(&s.z)
	s.r.standing.Bind(s.cols)
	s.query = qb.Build()
}

func (s *standingSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	hosted := !s.r.standing.Empty()
	var tick host.Tick
	if hosted {
		tick = s.r.tick.Of(d)
		tick.Places = s.r.places
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
			s.r.standing.Run(tick, s.cols.Row(cursor), len(cursor.IDs), s.about)
		}
	}
}

// the share of its top speed the i-th unit keeps: the ground under it, on its level, at the
// slope of the step ahead the way it goes, the one the planner priced
func (s *standingSystem) share(i int) float32 {
	b := &s.bases[i]
	at := b.Pos.Center()
	c, ok := s.r.grid.CellAt(at)
	if !ok {
		return 1
	}
	feet, reach := s.feet(i)
	kind, level := s.r.cells.Read().KindOn(c, at, feet, reach, s.r.level)
	step := cell.Step{From: c, To: c, Level: level, Kind: kind, Domain: unit.DomainAt(s.movers, i)}
	if !kind.Graded && s.r.rise != nil {
		dir := b.Vel.Dir
		if b.Vel.Value < 0 { // backing away: up or down the way it goes, not the way it faces
			dir = geom.NewVec(-dir.X, -dir.Y)
		}
		if n, ok := s.r.grid.Toward(c, s.nearest(dir)); ok {
			step.Slope = s.r.rise(c, n)
		}
	}
	return float32(cell.PaceOf(s.r.Mobility, step))
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
	cells := s.r.cells.Read()
	feet, reach := s.feet(i)
	level := float32(0)
	if s.r.level != nil {
		level = s.r.level(c)
	}
	return unit.Standing{ID: s.ids[i], Cell: c, Kind: cells.KindFor(c, at, feet, reach, s.r.level),
		States: cells.States(c), Box: s.bases[i].Pos.AABB.AABB, Heading: s.bases[i].Vel.Dir,
		Domain:   unit.DomainAt(s.movers, i),
		Airborne: s.states != nil && s.states[i].Has(unit.Airborne), Feet: feet, Level: level,
		Reach: reach}
}

func (s *standingSystem) feet(i int) (float32, float32) {
	if s.zs == nil {
		return f32.Inf(-1), 0
	}
	return s.zs[i].Altitude, s.zs[i].Reach()
}
