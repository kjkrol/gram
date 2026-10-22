package routing

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
)

// Coordinator answers the orders' asks (MoveOrder.Ask): a route to the Target, or the nearest
// free cell round it; an Update of d for a unit that has waited d more.
type Coordinator struct {
	keep spacing.Keeping

	query *goke.Query
	cell  goke.Comp[unit.At]
	order goke.Comp[order.MoveOrder]
	mover goke.OptComp[unit.Mover]
	base  goke.OptComp[world.Base]
	z     goke.OptComp[world.Z]
}

var _ goke.System = (*Coordinator)(nil)

// NewCoordinator answers the asks through k, the routing's own.
func NewCoordinator(k spacing.Keeping) *Coordinator { return &Coordinator{keep: k} }

func (c *Coordinator) Init(si *goke.SysInit) {
	c.query = si.NewQueryBuilder(&c.cell, &c.order).Optional(&c.mover).Optional(&c.base).
		Optional(&c.z).Build()
}

func (c *Coordinator) Update(_ *goke.CmdBuf, d time.Duration) {
	for c.query.All(); c.query.Next(); {
		cursor := c.query.Cursor()
		orders := c.order.Slice(cursor)
		cells, movers := c.cell.Slice(cursor), c.mover.Slice(cursor)
		bases, zs := c.base.Slice(cursor), c.z.Slice(cursor)
		for i, id := range cursor.IDs {
			o := &orders[i]
			if o.Ask != order.AskRoute && o.Ask != order.AskNearest {
				continue
			}
			m := bodies.Member{ID: id, Cell: cells[i].Cell, From: cells[i].Cell, Leg: o.Leg,
				Domain: unit.DomainAt(movers, i), Order: o}
			if bases != nil {
				m.Pos, m.Vel, m.Facing = bases[i].Pos, bases[i].Vel.Delta(), bases[i].Vel.Dir
			}
			if zs != nil {
				m.Z = zs[i]
			}
			if movers != nil {
				m.Lift = movers[i].Lift
			}
			c.answer(m, o, d)
		}
	}
}

func (c *Coordinator) answer(m bodies.Member, o *order.MoveOrder, d time.Duration) {
	if o.Ask == order.AskRoute {
		if path, found := c.keep.Route(m, m.From, o); found {
			o.Path, o.Waited, o.Ask = path, 0, order.AskNone
			return
		}
		if o.Waited += d; o.Waited < spacing.TargetWaitTimeout {
			o.Ask = order.NoRoute
			return
		}
	}
	dest, path, _, ok := c.keep.Lost(m, m.From, o, o.Waited)
	if !ok {
		o.Ask = order.NoPlace
		return
	}
	o.Target, o.Path, o.Waited, o.Ask = dest, path, 0, order.AskNone
	order.Found(o)
}
