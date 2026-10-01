package navigation

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// moveCommandSystem carries out MoveTo commands: every Selected entity of the player who gave one
// (owner.Obeys) — or the entity that gave it itself — whose domain the target takes gets its order
// as the keeping says — a free cell each, or a spot round the point — or with Append the target
// queued behind the order in flight. A LookAt has every such entity stop and turn.
type moveCommandSystem struct {
	keep     keeping
	moves    *control.Queue[MoveTo]
	looks    *control.Queue[LookAt]
	selected tag.Tag[selection.Family]
	kind     func(board.CellID) board.CellKind

	group   uint32 // the last group a MoveTo was given; found in the orders and LastOrders at the first
	grouped bool

	query   *goke.Query
	cell    goke.Comp[board.At]
	marks   goke.Comp[tag.Tags[selection.Family]]
	owners  goke.OptComp[tag.Tags[owner.Family]]
	order   goke.OptComp[MoveOrder]
	mover   goke.OptComp[board.Mover]
	base    goke.OptComp[world.Base]
	z       goke.OptComp[world.Z]
	steer   goke.OptComp[steering.Steering]
	orderID goke.CompID

	// self finds an entity that gives itself an order, selected or not
	self      *goke.Query
	selfCell  goke.Comp[board.At]
	selfOrder goke.OptComp[MoveOrder]
	selfMover goke.OptComp[board.Mover]
	selfBase  goke.OptComp[world.Base]
	selfZ     goke.OptComp[world.Z]
	selfSteer goke.OptComp[steering.Steering]
	selfLast  goke.OptComp[LastOrder]
}

// issuer is who gave a command: a player, or an entity for itself.
type issuer struct {
	player control.PlayerID
	entity uid.UID64
	self   bool
}

func issuedBy[C any](i control.Issued[C]) issuer {
	return issuer{player: i.Player, entity: i.Entity, self: i.ByEntity}
}

var _ goke.System = (*moveCommandSystem)(nil)

// newMoveCommandSystem builds a moveCommandSystem draining moves and looks into orders, a cell each
// through pathFinder.
func newMoveCommandSystem(pathFinder *pathFinder, moves *control.Queue[MoveTo], looks *control.Queue[LookAt], selected tag.Tag[selection.Family]) *moveCommandSystem {
	return &moveCommandSystem{keep: newCellKeeping(pathFinder, pathFinder.occupancy), moves: moves, looks: looks, selected: selected, kind: pathFinder.terrain.Kind}
}

// withKeeping has the system give its orders as k says.
func (s *moveCommandSystem) withKeeping(k keeping) *moveCommandSystem {
	s.keep = k
	return s
}

func (s *moveCommandSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.cell, &s.marks).Optional(&s.order).Optional(&s.mover).Optional(&s.base).Optional(&s.z).Optional(&s.steer).Optional(&s.owners).Build()
	s.orderID = si.RegComp[MoveOrder]()
	s.self = si.NewQueryBuilder(&s.selfCell).Optional(&s.selfOrder).Optional(&s.selfMover).Optional(&s.selfBase).Optional(&s.selfZ).Optional(&s.selfSteer).Optional(&s.selfLast).Build()
}

func (s *moveCommandSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.moves.Drain(func(i control.Issued[MoveTo]) { s.carryOut(cb, i.Command, issuedBy(i)) })
	s.looks.Drain(func(i control.Issued[LookAt]) { s.look(cb, i.Command.At, issuedBy(i)) })
}

// members calls fn with whom a command by gave concerns: the entity that gave it itself, or every
// Selected entity the player owns.
func (s *moveCommandSystem) members(by issuer, fn func(member)) {
	if !by.self {
		s.selectedMembers(by.player, fn)
		return
	}
	if !s.self.Seek(by.entity) {
		return
	}
	cur := s.self.Cursor()
	c := s.selfCell.At(cur)
	m := member{id: by.entity, cell: c.Cell, from: c.Cell, domain: board.DomainAt(nil, 0)}
	if o := s.selfOrder.At(cur); o != nil {
		m.order = o
		if o.Leg.Active {
			m.leg, m.from = o.Leg, o.Leg.To
		}
	}
	if mv := s.selfMover.At(cur); mv != nil {
		m.domain, m.lift = mv.Domain, mv.Lift
	}
	if b := s.selfBase.At(cur); b != nil {
		m.pos, m.vel, m.facing = b.Pos, b.Vel.Delta(), b.Vel.Dir
	}
	if z := s.selfZ.At(cur); z != nil {
		m.z = *z
	}
	if st := s.selfSteer.At(cur); st != nil {
		m.brake = st.Braking()
	}
	fn(m)
}

// selectedMembers calls fn with every Selected entity player by owns as a member.
func (s *moveCommandSystem) selectedMembers(by control.PlayerID, fn func(member)) {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		cells, marks, orders := s.cell.Slice(cursor), s.marks.Slice(cursor), s.order.Slice(cursor)
		movers, bases, zs, steers := s.mover.Slice(cursor), s.base.Slice(cursor), s.z.Slice(cursor), s.steer.Slice(cursor)
		owners := s.owners.Slice(cursor)
		for i, id := range cursor.IDs {
			var owned tag.Tags[owner.Family]
			if owners != nil {
				owned = owners[i]
			}
			if !marks[i].Has(s.selected) || !owner.Obeys(owned, by) {
				continue
			}
			m := member{id: id, cell: cells[i].Cell, from: cells[i].Cell, domain: board.DomainAt(movers, i)}
			if orders != nil {
				m.order = &orders[i]
				if leg := orders[i].Leg; leg.Active {
					m.leg, m.from = leg, leg.To
				}
			}
			if bases != nil {
				m.pos, m.vel, m.facing = bases[i].Pos, bases[i].Vel.Delta(), bases[i].Vel.Dir
			}
			if zs != nil {
				m.z = zs[i]
			}
			if movers != nil {
				m.lift = movers[i].Lift
			}
			if steers != nil {
				m.brake = steers[i].Braking()
			}
			fn(m)
		}
	}
}

// look has the members by concerns stop and turn towards at.
func (s *moveCommandSystem) look(cb *goke.CmdBuf, at geom.Vec, by issuer) {
	s.members(by, func(m member) { cb.AddOne(m.id, s.orderID, s.keep.look(m, at)) })
}

// carryOut gives the members by concerns whose domain cmd.Cell takes their orders toward it.
func (s *moveCommandSystem) carryOut(cb *goke.CmdBuf, cmd MoveTo, by issuer) {
	at := s.kind(cmd.Cell)
	var members []member
	s.members(by, func(m member) {
		if at.Admits(m.domain) {
			members = append(members, m)
		}
	})
	group := s.nextGroup()
	s.keep.orders(members, cmd, func(m member, o MoveOrder) {
		o.Group = group
		cb.AddOne(m.id, s.orderID, o)
	})
}

// nextGroup is a group no order has, nor any unit came to the end of: one past the highest found in
// the orders and the LastOrders at the first — a loaded game's too.
func (s *moveCommandSystem) nextGroup() uint32 {
	if !s.grouped {
		for s.self.All(); s.self.Next(); {
			cursor := s.self.Cursor()
			for _, o := range s.selfOrder.Slice(cursor) {
				s.group = max(s.group, o.Group)
			}
			for _, l := range s.selfLast.Slice(cursor) {
				s.group = max(s.group, l.Group)
			}
		}
		s.grouped = true
	}
	s.group++
	return s.group
}
