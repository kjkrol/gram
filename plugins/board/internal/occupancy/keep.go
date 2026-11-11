package occupancy

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// KeepSystem keeps o true to the world: every step it lets go of the holds of whoever left the
// world and has every unit pace says, an option of its query (nil, all), hold the cell it stands
// on, on the level its feet are on, the one its footprint on g is laid from; a unit travelling the
// abstract graph (sector.Abstract) or an aggregate its parts stand for (world.Unfolded) holds
// none. As it starts every unit holds the cell it is At.
func KeepSystem(o cell.Occupancy, footing Footing, g grid.Grid, pace goke.Opt) goke.System {
	s := &keepSystem{occupancy: o, footing: footing, grid: g, span: g.CellSpan(), pace: pace}
	s.gone, s.holding = s.leaving.gone, s.isHolding
	return s
}

// leaving tells by identity whoever may hold no cell: gone from the world, or an aggregate its
// parts stand for (world.Unfolded), read as a column since a Seek ignores a query's Exclude
type leaving struct {
	q        *goke.Query
	base     goke.Comp[world.Base]
	unfolded goke.OptComp[world.Unfolded]
}

func (l *leaving) init(si *goke.SysInit) {
	l.q = si.NewQueryBuilder(&l.base).Optional(&l.unfolded).Build()
}

func (l *leaving) gone(id uid.UID64) bool {
	return !l.q.Seek(id) || l.unfolded.Present(l.q.Cursor())
}

// Footing is the cell whoever stands at p, its feet at feet, stepping reach high, is on, whether on
// the raised deck over it, and whether it has a raised deck at all; false off the board.
type Footing func(p geom.Vec, feet, reach float32) (c cell.ID, level cell.Level, raised, ok bool)

var _ goke.System = (*keepSystem)(nil)

type keepSystem struct {
	occupancy cell.Occupancy
	footing   Footing
	pace      goke.Opt
	units     *goke.Query // who is still in the world, and where the units stand
	base      goke.Comp[world.Base]
	at        goke.OptComp[unit.At]
	mover     goke.OptComp[unit.Mover]
	z         goke.OptComp[world.Z]
	states    goke.OptComp[tag.Tags[unit.States]]
	statesID  goke.CompID
	lacking   []uid.UID64          // the units entering a cell whose chunk had no unit.States
	gone      func(uid.UID64) bool // isGone, bound once
	holding   func(uid.UID64, cell.Domain, cell.Level)
	asked     uid.UID64 // whom holding looks for
	held      bool      // whether holding found it
	grid      grid.Grid
	span      float32 // a cell's, past which a unit's footprint may be laid off its middle
	leaving   leaving
}

func (s *keepSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base).Optional(&s.at).Optional(&s.mover).Optional(&s.z).
		Optional(&s.states).
		Exclude(goke.Exclude[sector.Abstract](), goke.Exclude[world.Unfolded]())
	if s.pace != nil {
		qb.With(s.pace)
	}
	s.units = qb.Build()
	s.leaving.init(si)
	s.statesID = si.RegComp[tag.Tags[unit.States]]()
	if s.occupancy != nil {
		s.keep(s.units.All(), true)
	}
}

func (s *keepSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	if s.occupancy == nil {
		return
	}
	s.occupancy.Release(s.gone)
	s.keep(s.units.Paced(d), false)
	for _, id := range s.lacking { // no batch here: a move by id at once
		cb.AddOne(id, s.statesID, tag.Tags[unit.States](0).With(unit.Entered))
	}
	s.lacking = s.lacking[:0]
}

// q is the units' query, its walk begun
func (s *keepSystem) keep(q *goke.Query, starting bool) {
	for q.Next() {
		cur := q.Cursor()
		ats := s.at.Slice(cur)
		if ats == nil {
			continue
		}
		movers, bases, zs, states := s.mover.Slice(cur), s.base.Slice(cur), s.z.Slice(cur),
			s.states.Slice(cur)
		for i, id := range cur.IDs {
			feet, reach := f32.Inf(-1), float32(0)
			if zs != nil {
				feet, reach = zs[i].Altitude, zs[i].Reach()
			}
			p := bases[i].Pos.Center()
			if size := bases[i].Pos.Size; max(size.X, size.Y) > s.span {
				p = unit.LaidFrom(s.grid, size, p)
			}
			c, level, raised, ok := s.footing(p, feet, reach)
			if !ok {
				continue
			}
			at, domain := &ats[i], unit.DomainAt(movers, i)
			switch {
			case starting:
				s.occupancy.Enter(at.Cell, startOn(level, c == at.Cell), id, domain)
			case !s.holds(c, id):
				s.occupancy.Enter(c, level, id, domain)
				if c != at.Cell {
					s.occupancy.Leave(at.Cell, id)
					at.Cell, at.Next = c, 0
					s.enter(states, i, id)
				}
			case raised:
				s.occupancy.Enter(c, level, id, domain)
			}
		}
	}
}

func (s *keepSystem) holds(c cell.ID, id uid.UID64) bool {
	s.asked, s.held = id, false
	s.occupancy.Holders(c, s.holding)
	return s.held
}

func (s *keepSystem) isHolding(e uid.UID64, _ cell.Domain, _ cell.Level) {
	if e == s.asked {
		s.held = true
	}
}

// a unit whose chunk has no family gets it once the chunks are done
func (s *keepSystem) enter(states []tag.Tags[unit.States], i int, id uid.UID64) {
	if states != nil {
		states[i] = states[i].With(unit.Entered)
	} else {
		s.lacking = append(s.lacking, id)
	}
}

// a unit starting holds its own cell on the level it stands on there, on the ground where the
// floor it stands on is another cell's
func startOn(level cell.Level, own bool) cell.Level {
	if own {
		return level
	}
	return cell.OnGround
}
