package navigation

import (
	"cmp"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/uid"
)

// moveCommandSystem carries out MoveTo commands: each gives every Selected entity its own free
// cell at or around the target, nearest entity first, or with Append queues the target behind an
// order already in flight; one standing on the target turns towards the point clicked instead. A
// LookAt has every Selected entity finish its step, stop and turn.
type moveCommandSystem struct {
	pathFinder *pathFinder
	moves      *control.Inbox[MoveTo]
	looks      *control.Inbox[LookAt]
	selected   plugin.Tag[selection.Family]

	query   *goke.Query
	cell    goke.Comp[board.Cell]
	marks   goke.Comp[plugin.Tags[selection.Family]]
	order   goke.OptComp[MoveOrder]
	mover   goke.OptComp[board.Mover]
	orderID goke.CompID
}

var _ goke.System = (*moveCommandSystem)(nil)

// newMoveCommandSystem builds a moveCommandSystem draining moves and looks into orders via pathFinder.
func newMoveCommandSystem(pathFinder *pathFinder, moves *control.Inbox[MoveTo], looks *control.Inbox[LookAt], selected plugin.Tag[selection.Family]) *moveCommandSystem {
	return &moveCommandSystem{moves: moves, looks: looks, pathFinder: pathFinder, selected: selected}
}

func (s *moveCommandSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.cell, &s.marks).Optional(&s.order).Optional(&s.mover).Build()
	s.orderID = si.RegComp[MoveOrder]()
}

func (s *moveCommandSystem) Update(cb *goke.CmdBuf, _ time.Duration) {
	s.moves.Drain(func(i control.Issued[MoveTo]) { s.carryOut(cb, i.Command) })
	s.looks.Drain(func(i control.Issued[LookAt]) { s.look(cb, i.Command.At) })
}

// look has every Selected entity stop where its step ends and turn towards at.
func (s *moveCommandSystem) look(cb *goke.CmdBuf, at geom.Vec) {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		cells, marks, orders := s.cell.Slice(cursor), s.marks.Slice(cursor), s.order.Slice(cursor)
		for i, id := range cursor.IDs {
			if !marks[i].Has(s.selected) {
				continue
			}
			order := MoveOrder{Target: cells[i].ID, Face: at}
			if orders != nil && orders[i].Leg.Active {
				order.Target, order.Leg = orders[i].Leg.To, orders[i].Leg
			}
			cb.AddOne(id, s.orderID, order)
		}
	}
}

// carryOut gives the Selected entities their orders toward cmd.Cell.
func (s *moveCommandSystem) carryOut(cb *goke.CmdBuf, cmd MoveTo) {
	target := cmd.Cell
	pf := s.pathFinder
	at := pf.terrain.Kind(target)

	var moves []pendingMove
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		cells := s.cell.Slice(cursor)
		marks := s.marks.Slice(cursor)
		orders := s.order.Slice(cursor)
		movers := s.mover.Slice(cursor)
		for i, id := range cursor.IDs {
			if !marks[i].Has(s.selected) {
				continue
			}
			domain := board.DomainAt(movers, i)
			if !at.Admits(domain) {
				continue
			}
			if cmd.Append && orders != nil {
				orders[i].Enqueue(target)
				continue
			}
			standing := orders == nil || !orders[i].Leg.Active
			if !cmd.Append && standing && cells[i].ID == target && cmd.At != (geom.Vec{}) {
				// clicked where it stands: stay and turn towards the point
				cb.AddOne(id, s.orderID, MoveOrder{Target: target, Face: cmd.At})
				continue
			}
			m := pendingMove{id: id, from: cells[i].ID, domain: domain}
			if orders != nil && orders[i].Leg.Active {
				m.leg, m.from = orders[i].Leg, orders[i].Leg.To
			}
			moves = append(moves, m)
		}
	}
	slices.SortStableFunc(moves, func(a, b pendingMove) int {
		return cmp.Compare(pf.grid.Distance(a.from, target), pf.grid.Distance(b.from, target))
	})

	taken := make(map[board.CellID]bool)
	for n, m := range moves {
		dest := target
		var path Path
		ok := false
		if n == 0 {
			path, ok = pf.findPath(m.id, m.domain, m.from, target)
		}
		if !ok {
			dest, path, ok = pf.nearestFree(m.id, m.domain, m.from, target, taken)
		}
		if !ok {
			continue
		}
		taken[dest] = true
		cb.AddOne(m.id, s.orderID, MoveOrder{Target: dest, Path: path, Leg: m.leg})
	}
}

// pendingMove is one Selected entity awaiting a destination.
type pendingMove struct {
	id     uid.UID64
	from   board.CellID
	leg    Leg
	domain board.Domain
}
