package routing

import (
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
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

type System struct {
	keep     spacing.Keeping
	moves    *command.Queue[order.MoveTo]
	looks    *command.Queue[order.LookAt]
	Onwards  *command.Queue[order.Onward]
	selected tag.Tag[selection.Family]
	kind     func(cell.ID) cell.Kind

	group   uint32
	grouped bool

	query   *goke.Query
	cell    goke.Comp[unit.At]
	marks   goke.Comp[tag.Tags[selection.Family]]
	owners  goke.OptComp[tag.Tags[owner.Family]]
	order   goke.OptComp[order.MoveOrder]
	mover   goke.OptComp[unit.Mover]
	base    goke.OptComp[world.Base]
	z       goke.OptComp[world.Z]
	steer   goke.OptComp[steering.Steering]
	orderID goke.CompID

	self      *goke.Query
	selfCell  goke.Comp[unit.At]
	selfOrder goke.OptComp[order.MoveOrder]
	selfMover goke.OptComp[unit.Mover]
	selfBase  goke.OptComp[world.Base]
	selfZ     goke.OptComp[world.Z]
	selfSteer goke.OptComp[steering.Steering]
	selfLast  goke.OptComp[order.LastOrder]
	selfRound goke.OptComp[order.Round]

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

var _ goke.System = (*System)(nil)

func NewSystem(pathFinder *spacing.Finder, moves *command.Queue[order.MoveTo], looks *command.Queue[order.LookAt], selected tag.Tag[selection.Family]) *System {
	return &System{keep: spacing.NewCellKeeping(pathFinder, pathFinder.Occupancy), moves: moves, looks: looks, selected: selected, kind: pathFinder.Terrain.Kind}
}

func (s *System) WithKeeping(k spacing.Keeping) *System {
	s.keep = k
	return s
}

func (s *System) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.cell, &s.marks).Optional(&s.order).Optional(&s.mover).Optional(&s.base).Optional(&s.z).Optional(&s.steer).Optional(&s.owners).Build()
	s.orderID = si.RegComp[order.MoveOrder]()
	s.self = si.NewQueryBuilder(&s.selfCell).
		Optional(&s.selfOrder).Optional(&s.selfMover).Optional(&s.selfBase).Optional(&s.selfZ).
		Optional(&s.selfSteer).Optional(&s.selfLast).Optional(&s.selfRound).Build()
	s.named = si.NewQueryBuilder(&s.label).Optional(&s.plot).Optional(&s.at).Build()
}

func (s *System) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.moves.Drain(func(i command.Issued[order.MoveTo]) { s.carryOut(cb, i.Command, issuedBy(i)) })
	s.looks.Drain(func(i command.Issued[order.LookAt]) { s.look(cb, i.Command.At, issuedBy(i)) })
	if s.Onwards != nil {
		s.Onwards.Drain(func(i command.Issued[order.Onward]) {
			if i.ByEntity {
				s.onward(cb, i.Entity)
			}
		})
	}
}

func (s *System) onward(cb *goke.CmdBuf, id uid.UID64) {
	if !s.self.Seek(id) {
		return
	}
	r := s.selfRound.At(s.self.Cursor())
	if r == nil || r.Count == 0 {
		return
	}
	next := r.Cells[r.Next%r.Count]
	r.Next = (r.Next + 1) % r.Count
	s.carryOut(cb, order.MoveTo{Cell: next}, issuer{entity: id, self: true})
}

// the cell of the one entity bearing a name To gives: a cell's own, or the one a unit is at
func (s *System) find(to ceremony.Whom) (cell.ID, bool) {
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

func (s *System) members(by issuer, fn func(bodies.Member)) {
	if !by.self {
		s.selectedMembers(by.player, fn)
		return
	}
	if !s.self.Seek(by.entity) {
		return
	}
	cur := s.self.Cursor()
	c := s.selfCell.At(cur)
	m := bodies.Member{ID: by.entity, Cell: c.Cell, From: c.Cell, Domain: unit.DomainAt(nil, 0)}
	if o := s.selfOrder.At(cur); o != nil {
		m.Order = o
		if o.Leg.Active {
			m.Leg, m.From = o.Leg, o.Leg.To
		}
	}
	if mv := s.selfMover.At(cur); mv != nil {
		m.Domain, m.Lift = mv.Domain, mv.Lift
	}
	if b := s.selfBase.At(cur); b != nil {
		m.Pos, m.Vel, m.Facing = b.Pos, b.Vel.Delta(), b.Vel.Dir
	}
	if z := s.selfZ.At(cur); z != nil {
		m.Z = *z
	}
	if st := s.selfSteer.At(cur); st != nil {
		m.Brake = st.Braking()
	}
	fn(m)
}

func (s *System) selectedMembers(by command.Issuer, fn func(bodies.Member)) {
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
			m := bodies.Member{ID: id, Cell: cells[i].Cell, From: cells[i].Cell, Domain: unit.DomainAt(movers, i)}
			if orders != nil {
				m.Order = &orders[i]
				if leg := orders[i].Leg; leg.Active {
					m.Leg, m.From = leg, leg.To
				}
			}
			if bases != nil {
				m.Pos, m.Vel, m.Facing = bases[i].Pos, bases[i].Vel.Delta(), bases[i].Vel.Dir
			}
			if zs != nil {
				m.Z = zs[i]
			}
			if movers != nil {
				m.Lift = movers[i].Lift
			}
			if steers != nil {
				m.Brake = steers[i].Braking()
			}
			fn(m)
		}
	}
}

func (s *System) look(cb *goke.CmdBuf, at geom.Vec, by issuer) {
	s.members(by, func(m bodies.Member) { cb.AddOne(m.ID, s.orderID, s.keep.Look(m, at)) })
}

func (s *System) carryOut(cb *goke.CmdBuf, cmd order.MoveTo, by issuer) {
	if !cmd.To.Nobody() {
		c, ok := s.find(cmd.To)
		if !ok {
			return
		}
		cmd.Cell, cmd.To = c, ceremony.Whom{}
	}
	at := s.kind(cmd.Cell)
	var members []bodies.Member
	s.members(by, func(m bodies.Member) {
		if at.Admits(m.Domain) {
			members = append(members, m)
		}
	})
	group := s.nextGroup()
	s.keep.Orders(members, cmd, func(m bodies.Member, o order.MoveOrder) {
		o.Group = group
		cb.AddOne(m.ID, s.orderID, o)
	})
}

// one past the highest group in the orders and LastOrders at the first: a loaded game's too
func (s *System) nextGroup() uint32 {
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
