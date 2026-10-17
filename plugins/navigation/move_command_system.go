package navigation

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type moveCommandSystem struct {
	keep     keeping
	moves    *command.Queue[MoveTo]
	looks    *command.Queue[LookAt]
	onwards  *command.Queue[Onward]
	selected tag.Tag[selection.Family]
	kind     func(cell.ID) cell.Kind

	group   uint32
	grouped bool

	query   *goke.Query
	cell    goke.Comp[unit.At]
	marks   goke.Comp[tag.Tags[selection.Family]]
	owners  goke.OptComp[tag.Tags[owner.Family]]
	order   goke.OptComp[MoveOrder]
	mover   goke.OptComp[unit.Mover]
	base    goke.OptComp[world.Base]
	z       goke.OptComp[world.Z]
	steer   goke.OptComp[steering.Steering]
	orderID goke.CompID

	self      *goke.Query
	selfCell  goke.Comp[unit.At]
	selfOrder goke.OptComp[MoveOrder]
	selfMover goke.OptComp[unit.Mover]
	selfBase  goke.OptComp[world.Base]
	selfZ     goke.OptComp[world.Z]
	selfSteer goke.OptComp[steering.Steering]
	selfLast  goke.OptComp[LastOrder]
	selfRound goke.OptComp[Round]

	named *goke.Query
	label goke.Comp[ceremony.Label]
	plot  goke.OptComp[cell.Plot]
	at    goke.OptComp[unit.At]
}

type issuer struct {
	player command.Issuer
	entity uid.UID64
	self   bool
}

func issuedBy[C command.Command](i command.Issued[C]) issuer {
	return issuer{player: i.Issuer, entity: i.Entity, self: i.ByEntity}
}

var _ goke.System = (*moveCommandSystem)(nil)

func newMoveCommandSystem(pathFinder *pathFinder, moves *command.Queue[MoveTo], looks *command.Queue[LookAt], selected tag.Tag[selection.Family]) *moveCommandSystem {
	return &moveCommandSystem{keep: newCellKeeping(pathFinder, pathFinder.occupancy), moves: moves, looks: looks, selected: selected, kind: pathFinder.terrain.Kind}
}

func (s *moveCommandSystem) withKeeping(k keeping) *moveCommandSystem {
	s.keep = k
	return s
}

func (s *moveCommandSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.cell, &s.marks).Optional(&s.order).Optional(&s.mover).Optional(&s.base).Optional(&s.z).Optional(&s.steer).Optional(&s.owners).Build()
	s.orderID = si.RegComp[MoveOrder]()
	s.self = si.NewQueryBuilder(&s.selfCell).
		Optional(&s.selfOrder).Optional(&s.selfMover).Optional(&s.selfBase).Optional(&s.selfZ).
		Optional(&s.selfSteer).Optional(&s.selfLast).Optional(&s.selfRound).Build()
	s.named = si.NewQueryBuilder(&s.label).Optional(&s.plot).Optional(&s.at).Build()
}

func (s *moveCommandSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.moves.Drain(func(i command.Issued[MoveTo]) { s.carryOut(cb, i.Command, issuedBy(i)) })
	s.looks.Drain(func(i command.Issued[LookAt]) { s.look(cb, i.Command.At, issuedBy(i)) })
	if s.onwards != nil {
		s.onwards.Drain(func(i command.Issued[Onward]) {
			if i.ByEntity {
				s.onward(cb, i.Entity)
			}
		})
	}
}

func (s *moveCommandSystem) onward(cb *goke.CmdBuf, id uid.UID64) {
	if !s.self.Seek(id) {
		return
	}
	r := s.selfRound.At(s.self.Cursor())
	if r == nil || r.Count == 0 {
		return
	}
	next := r.Cells[r.Next%r.Count]
	r.Next = (r.Next + 1) % r.Count
	s.carryOut(cb, MoveTo{Cell: next}, issuer{entity: id, self: true})
}

// the cell of the one entity bearing a name To gives: a cell's own, or the one a unit is at
func (s *moveCommandSystem) find(to ceremony.Whom) (cell.ID, bool) {
	var found cell.ID
	n := 0
	for s.named.All(); s.named.Next(); {
		cur := s.named.Cursor()
		labels, plots, ats := s.label.Slice(cur), s.plot.Slice(cur), s.at.Slice(cur)
		for i := range cur.IDs {
			if !to.Holds(labels[i]) {
				continue
			}
			switch {
			case plots != nil:
				found = plots[i].Cell
			case ats != nil:
				found = ats[i].Cell
			default:
				continue
			}
			n++
		}
	}
	return found, n == 1
}

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
	m := member{id: by.entity, cell: c.Cell, from: c.Cell, domain: unit.DomainAt(nil, 0)}
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

func (s *moveCommandSystem) selectedMembers(by command.Issuer, fn func(member)) {
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
			m := member{id: id, cell: cells[i].Cell, from: cells[i].Cell, domain: unit.DomainAt(movers, i)}
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

func (s *moveCommandSystem) look(cb *goke.CmdBuf, at geom.Vec, by issuer) {
	s.members(by, func(m member) { cb.AddOne(m.id, s.orderID, s.keep.look(m, at)) })
}

func (s *moveCommandSystem) carryOut(cb *goke.CmdBuf, cmd MoveTo, by issuer) {
	if !cmd.To.Nobody() {
		c, ok := s.find(cmd.To)
		if !ok {
			return
		}
		cmd.Cell, cmd.To = c, ceremony.Whom{}
	}
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

// one past the highest group in the orders and LastOrders at the first: a loaded game's too
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
