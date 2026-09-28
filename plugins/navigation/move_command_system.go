package navigation

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

// moveCommandSystem carries out MoveTo commands: every Selected entity whose domain the target
// takes gets its order as the keeping says — a free cell each, or a spot round the point — or with
// Append the target queued behind the order in flight. A LookAt has every Selected entity stop and
// turn.
type moveCommandSystem struct {
	keep     keeping
	moves    *control.Queue[MoveTo]
	looks    *control.Queue[LookAt]
	selected plugin.Tag[selection.Family]
	kind     func(board.CellID) board.CellKind

	query   *goke.Query
	cell    goke.Comp[board.Cell]
	marks   goke.Comp[plugin.Tags[selection.Family]]
	order   goke.OptComp[MoveOrder]
	mover   goke.OptComp[board.Mover]
	base    goke.OptComp[world.Base]
	z       goke.OptComp[world.Z]
	steer   goke.OptComp[steering.Steering]
	orderID goke.CompID
}

var _ goke.System = (*moveCommandSystem)(nil)

// newMoveCommandSystem builds a moveCommandSystem draining moves and looks into orders, a cell each
// through pathFinder.
func newMoveCommandSystem(pathFinder *pathFinder, moves *control.Queue[MoveTo], looks *control.Queue[LookAt], selected plugin.Tag[selection.Family]) *moveCommandSystem {
	return &moveCommandSystem{keep: newCellKeeping(pathFinder), moves: moves, looks: looks, selected: selected, kind: pathFinder.terrain.Kind}
}

// withKeeping has the system give its orders as k says.
func (s *moveCommandSystem) withKeeping(k keeping) *moveCommandSystem {
	s.keep = k
	return s
}

func (s *moveCommandSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.cell, &s.marks).Optional(&s.order).Optional(&s.mover).Optional(&s.base).Optional(&s.z).Optional(&s.steer).Build()
	s.orderID = si.RegComp[MoveOrder]()
}

func (s *moveCommandSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.moves.Drain(func(i control.Issued[MoveTo]) { s.carryOut(cb, i.Command) })
	s.looks.Drain(func(i control.Issued[LookAt]) { s.look(cb, i.Command.At) })
}

// selectedMembers calls fn with every Selected entity as a member.
func (s *moveCommandSystem) selectedMembers(fn func(member)) {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		cells, marks, orders := s.cell.Slice(cursor), s.marks.Slice(cursor), s.order.Slice(cursor)
		movers, bases, zs, steers := s.mover.Slice(cursor), s.base.Slice(cursor), s.z.Slice(cursor), s.steer.Slice(cursor)
		for i, id := range cursor.IDs {
			if !marks[i].Has(s.selected) {
				continue
			}
			m := member{id: id, cell: cells[i].ID, from: cells[i].ID, domain: board.DomainAt(movers, i)}
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

// look has every Selected entity stop and turn towards at.
func (s *moveCommandSystem) look(cb *goke.CmdBuf, at geom.Vec) {
	s.selectedMembers(func(m member) { cb.AddOne(m.id, s.orderID, s.keep.look(m, at)) })
}

// carryOut gives the Selected entities whose domain cmd.Cell takes their orders toward it.
func (s *moveCommandSystem) carryOut(cb *goke.CmdBuf, cmd MoveTo) {
	at := s.kind(cmd.Cell)
	var members []member
	s.selectedMembers(func(m member) {
		if at.Admits(m.domain) {
			members = append(members, m)
		}
	})
	s.keep.orders(members, cmd, func(m member, o MoveOrder) { cb.AddOne(m.id, s.orderID, o) })
}
