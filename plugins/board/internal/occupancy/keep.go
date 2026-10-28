package occupancy

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// KeepSystem keeps o true to the world: every step it lets go of the holds of whoever left the
// world and has every unit hold the cell it stands on, on the level its feet are on. As it starts
// every unit holds the cell it is At.
func KeepSystem(o cell.Occupancy, footing Footing) goke.System {
	s := &keepSystem{occupancy: o, footing: footing}
	s.gone, s.holding = s.isGone, s.isHolding
	return s
}

// Footing is the cell whoever stands at p, its feet at feet, stepping reach high, is on, whether on
// the raised deck over it, and whether it has a raised deck at all; false off the board.
type Footing func(p geom.Vec, feet, reach float32) (c cell.ID, deck, raised, ok bool)

var _ goke.System = (*keepSystem)(nil)

type keepSystem struct {
	occupancy cell.Occupancy
	footing   Footing
	units     *goke.Query // who is still in the world, and where the units stand
	base      goke.Comp[world.Base]
	at        goke.OptComp[unit.At]
	mover     goke.OptComp[unit.Mover]
	z         goke.OptComp[world.Z]
	states    goke.OptComp[tag.Tags[unit.States]]
	statesID  goke.CompID
	lacking   []uid.UID64          // the units entering a cell whose chunk had no unit.States
	gone      func(uid.UID64) bool // isGone, bound once
	holding   func(uid.UID64, cell.Domain, bool)
	asked     uid.UID64 // whom holding looks for
	held      bool      // whether holding found it
}

func (s *keepSystem) Init(si *goke.SysInit) {
	s.units = si.NewQueryBuilder(&s.base).Optional(&s.at).Optional(&s.mover).Optional(&s.z).
		Optional(&s.states).Build()
	s.statesID = si.RegComp[tag.Tags[unit.States]]()
	if s.occupancy != nil {
		s.keep(true)
	}
}

func (s *keepSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	if s.occupancy == nil {
		return
	}
	s.occupancy.Release(s.gone)
	s.keep(false)
	for _, id := range s.lacking { // no batch here: a move by id at once
		cb.AddOne(id, s.statesID, tag.Tags[unit.States](0).With(unit.Entered))
	}
	s.lacking = s.lacking[:0]
}

func (s *keepSystem) keep(starting bool) {
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
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
			c, deck, raised, ok := s.footing(bases[i].Pos.Center(), feet, reach)
			if !ok {
				continue
			}
			at, domain := &ats[i], unit.DomainAt(movers, i)
			switch {
			case starting:
				s.occupancy.Enter(at.Cell, deck && c == at.Cell, id, domain)
			case !s.holds(c, id):
				s.occupancy.Enter(c, deck, id, domain)
				if c != at.Cell {
					s.occupancy.Leave(at.Cell, id)
					at.Cell, at.Next = c, 0
					s.enter(states, i, id)
				}
			case raised:
				s.occupancy.Enter(c, deck, id, domain)
			}
		}
	}
}

func (s *keepSystem) holds(c cell.ID, id uid.UID64) bool {
	s.asked, s.held = id, false
	s.occupancy.Holders(c, s.holding)
	return s.held
}

func (s *keepSystem) isHolding(e uid.UID64, _ cell.Domain, _ bool) {
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

func (s *keepSystem) isGone(id uid.UID64) bool { return !s.units.Seek(id) }
