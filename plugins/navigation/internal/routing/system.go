package routing

import (
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// System carries out the MoveTo, March, LookAt and Onward commands: an order for each unit sent;
// it keeps an aggregate its orders are aimed at ticked at L3 while one is.
type System struct {
	// Pace is the pace of the units a player's command sends, an option of their query; nil, all.
	Pace     goke.Opt
	keep     spacing.Keeping
	moves    *command.Queue[order.MoveTo]
	looks    *command.Queue[order.LookAt]
	Onwards  *command.Queue[order.Onward]
	Marches  *command.Queue[order.March]
	selected tag.Tag[selection.Family]
	kind     func(cell.ID) cell.Kind

	group     uint32
	grouped   bool
	shapeless bool // a March in a shape unknown here was said in the log

	query   *goke.Query
	cell    goke.Comp[unit.At]
	marks   goke.Comp[tag.Tags[selection.Family]]
	owners  goke.OptComp[tag.Tags[owner.Family]]
	order   goke.OptComp[order.MoveOrder]
	mover   goke.OptComp[unit.Mover]
	base    goke.OptComp[world.Base]
	z       goke.OptComp[world.Z]
	steer   goke.OptComp[steering.Steering]
	levels  bodies.Levels
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
	selfLevel bodies.Levels

	named *goke.Query
	label goke.Comp[ceremony.Label]
	plot  goke.OptComp[cell.Plot]
	at    goke.OptComp[unit.At]
	whole goke.OptComp[world.Aggregate]

	aims aims
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

// NewSystem carries out moves and looks for the units marked selected, routing over pathFinder.
func NewSystem(pathFinder *spacing.Finder, moves *command.Queue[order.MoveTo], looks *command.Queue[order.LookAt], selected tag.Tag[selection.Family]) *System {
	return &System{keep: spacing.NewCellKeeping(pathFinder, pathFinder.Occupancy), moves: moves, looks: looks, selected: selected, kind: pathFinder.Terrain.Kind}
}

// WithKeeping has k place the units a command sends.
func (s *System) WithKeeping(k spacing.Keeping) *System {
	s.keep = k
	return s
}

func (s *System) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.cell, &s.marks).Optional(&s.order).Optional(&s.mover).
		Optional(&s.base).Optional(&s.z).Optional(&s.steer).Optional(&s.owners)
	s.levels.Bind(qb)
	if s.Pace != nil {
		qb.With(s.Pace)
	}
	s.query = qb.Build()
	s.orderID = si.RegComp[order.MoveOrder]()
	self := si.NewQueryBuilder(&s.selfCell).
		Optional(&s.selfOrder).Optional(&s.selfMover).Optional(&s.selfBase).Optional(&s.selfZ).
		Optional(&s.selfSteer).Optional(&s.selfLast).Optional(&s.selfRound)
	s.self = s.selfLevel.Bind(self).Build()
	s.named = si.NewQueryBuilder(&s.label).Optional(&s.plot).Optional(&s.at).Optional(&s.whole).
		Build()
	s.aims.init(si)
}

func (s *System) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.moves.Drain(func(i command.Issued[order.MoveTo]) {
		s.carryOut(cb, i.Command, order.Formation{}, issuedBy(i))
	})
	if s.Marches != nil {
		s.Marches.Drain(func(i command.Issued[order.March]) { s.march(cb, i.Command, issuedBy(i)) })
	}
	s.looks.Drain(func(i command.Issued[order.LookAt]) { s.look(cb, i.Command.At, issuedBy(i)) })
	if s.Onwards != nil {
		s.Onwards.Drain(func(i command.Issued[order.Onward]) {
			if i.ByEntity {
				s.onward(cb, i.Entity)
			}
		})
	}
	s.aims.keep(cb)
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
	s.carryOut(cb, order.MoveTo{Cell: next}, order.Formation{}, issuer{entity: id, self: true})
}

// the one entity bearing a name To gives, the cell it names, a cell's own or the one a unit is
// at, and whether it is an aggregate
func (s *System) find(to ceremony.Whom) (uid.UID64, cell.ID, bool, bool) {
	var id uid.UID64
	var found cell.ID
	var whole bool
	n := 0
	for s.named.All(); s.named.Next(); {
		cur := s.named.Cursor()
		labels, plots, ats := s.label.Slice(cur), s.plot.Slice(cur), s.at.Slice(cur)
		for i, e := range cur.IDs {
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
			id, whole = e, s.whole.Present(cur)
			n++
		}
	}
	return id, found, whole, n == 1
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
	m := bodies.Member{ID: by.entity, Cell: c.Cell, From: c.Cell, Domain: unit.DomainAt(nil, 0),
		Level: s.selfLevel.Of(cur)}
	if o := s.selfOrder.At(cur); o != nil {
		m.Order = o
		if o.Leg.Active {
			m.Leg, m.From = o.Leg, o.Leg.To
		}
	}
	if mv := s.selfMover.At(cur); mv != nil {
		m.Domain, m.Lift, m.Weight = mv.Domain, mv.Lift, mv.Weight
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
	for s.query.Paced(0); s.query.Next(); {
		cursor := s.query.Cursor()
		cells, marks, orders := s.cell.Slice(cursor), s.marks.Slice(cursor), s.order.Slice(cursor)
		movers, bases, zs, steers := s.mover.Slice(cursor), s.base.Slice(cursor), s.z.Slice(cursor), s.steer.Slice(cursor)
		owners, level := s.owners.Slice(cursor), s.levels.Of(cursor)
		for i, id := range cursor.IDs {
			var owned tag.Tags[owner.Family]
			if owners != nil {
				owned = owners[i]
			}
			if !marks[i].Has(s.selected) || !owner.Obeys(owned, by) {
				continue
			}
			m := bodies.Member{ID: id, Cell: cells[i].Cell, From: cells[i].Cell,
				Domain: unit.DomainAt(movers, i), Level: level}
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
				m.Lift, m.Weight = movers[i].Lift, movers[i].Weight
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

func (s *System) march(cb *goke.CmdBuf, cmd order.March, by issuer) {
	in := cmd.In
	if in.Shape > order.Column {
		if !s.shapeless {
			log.Printf("navigation: a March in shape %d, unknown here, goes as a MoveTo", in.Shape)
			s.shapeless = true
		}
		in = order.Formation{}
	}
	s.carryOut(cb, order.MoveTo{Cell: cmd.Cell, To: cmd.To, At: cmd.At, Level: cmd.Level}, in, by)
}

func (s *System) carryOut(cb *goke.CmdBuf, cmd order.MoveTo, in order.Formation, by issuer) {
	var aim uid.UID64
	var aimed bool
	if !cmd.To.Nobody() {
		id, c, whole, ok := s.find(cmd.To)
		if !ok {
			return
		}
		cmd.Cell, cmd.To = c, ceremony.Whom{}
		aim, aimed = id, whole
	}
	at := s.kind(cmd.Cell)
	var members []bodies.Member
	s.members(by, func(m bodies.Member) {
		if at.Admits(m.Domain) {
			members = append(members, m)
		}
	})
	var queued []uint8
	if aimed && cmd.Append {
		queued = make([]uint8, len(members))
		for k, m := range members {
			if m.Order != nil {
				queued[k] = m.Order.Queued
			}
		}
	}
	group, party := s.nextGroup(), uint16(min(len(members), math.MaxUint16))
	s.keep.Orders(members, cmd, func(m bodies.Member, o order.MoveOrder) {
		o.Group, o.Party, o.Formation = group, party, in
		if aimed {
			o.Aim, o.Aimed = aim, true
			s.aims.mark(cb, m.ID)
		}
		cb.AddOne(m.ID, s.orderID, o)
	})
	if queued == nil {
		return
	}
	// a goal queued behind an order is aimed with it
	for k, m := range members {
		if m.Order != nil && m.Order.Queued > queued[k] {
			m.Order.Aim, m.Order.Aimed = aim, true
			s.aims.mark(cb, m.ID)
		}
	}
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
