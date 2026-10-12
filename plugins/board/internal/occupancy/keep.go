package occupancy

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// KeepSystem keeps o true to the world. As it starts — after a Populate as after a Load — every
// unit holds the cell it is At. Every step it lets go of the holds of whoever is no longer in the
// world, so one despawned — fallen in, say — blocks no cell, and has every unit hold the cell it
// stands on: one holding it nothing — spawned in the game, moved by none who keeps its cell —
// lets go of the cell it was At and is At the cell it stands on, with unit.Entered; one under a
// raised deck holds its cell on the level its feet are on, as it climbs onto the deck or falls off.
func KeepSystem(o cell.Occupancy, footing Footing) goke.System {
	s := &keepSystem{occupancy: o, footing: footing}
	s.gone, s.holding = s.isGone, s.isHolding
	return s
}

// Footing is the cell whoever stands at p, its feet at feet, stepping reach high, is on, whether on
// the raised deck over it, and whether it has a raised deck at all; false off the board.
type Footing func(p geom.Vec, feet, reach float64) (c cell.ID, deck, raised, ok bool)

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

// keep has every unit hold the cell it stands on, as it starts the cell it is At.
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
			feet, reach := math.Inf(-1), 0.0
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
					at.Cell = c
					s.enter(states, i, id)
				}
			case raised:
				s.occupancy.Enter(c, deck, id, domain)
			}
		}
	}
}

// holds reports whether id holds c, on either of its levels.
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

// enter has the unit at row i of states have unit.Entered on for this step; one whose chunk has no
// family gets it once the chunks are done.
func (s *keepSystem) enter(states []tag.Tags[unit.States], i int, id uid.UID64) {
	if states != nil {
		states[i] = states[i].With(unit.Entered)
	} else {
		s.lacking = append(s.lacking, id)
	}
}

// isGone reports whether id is no longer in the world.
func (s *keepSystem) isGone(id uid.UID64) bool { return !s.units.Seek(id) }
