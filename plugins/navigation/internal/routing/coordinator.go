package routing

import (
	"sync/atomic"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Coordinator answers the orders' asks (MoveOrder.Ask): a route to the Target, or the nearest
// free cell round it; an Update of d for a unit that has waited d more. The asks are shared
// among the pool's goroutines, each searching through one of the routers at a time.
type Coordinator struct {
	pool    *parallel.Pool
	read    func(w int) *board.Read // nil: the routers read what they were made over
	routers []router
	start   int         // the router the first goroutine takes, one on every Update
	asks    []uid.UID64 // the units that asked
	next    atomic.Int64
	d       time.Duration
	task    func(w, i int) // the method value, bound once

	query *goke.Query
	forks []*goke.Query // by goroutine, query the first
	cell  goke.Comp[unit.At]
	order goke.Comp[order.MoveOrder]
	mover goke.OptComp[unit.Mover]
	base  goke.OptComp[world.Base]
	z     goke.OptComp[world.Z]
}

type router struct {
	finder *spacing.Finder
	keep   spacing.Keeping
}

var _ goke.System = (*Coordinator)(nil)

// NewCoordinator answers the asks on pool's goroutines, nil the caller's alone; on goroutine w a
// router searches the board through read(w).
func NewCoordinator(pool *parallel.Pool, read func(w int) *board.Read) *Coordinator {
	return &Coordinator{pool: pool, read: read}
}

// Router adds a router: keep, searching through f.
func (c *Coordinator) Router(f *spacing.Finder, keep spacing.Keeping) *Coordinator {
	c.routers = append(c.routers, router{finder: f, keep: keep})
	return c
}

func (c *Coordinator) Init(si *goke.SysInit) {
	c.query = si.NewQueryBuilder(&c.cell, &c.order).Optional(&c.mover).Optional(&c.base).
		Optional(&c.z).Build()
	c.forks = append(c.forks[:0], c.query)
	for range c.pool.Size() - 1 {
		c.forks = append(c.forks, c.query.Fork())
	}
	c.task = c.route
}

func (c *Coordinator) Update(_ *goke.CmdBuf, d time.Duration) {
	c.asks = c.asks[:0]
	for c.query.All(); c.query.Next(); {
		cursor := c.query.Cursor()
		orders := c.order.Slice(cursor)
		for i := range orders {
			if a := orders[i].Ask; a == order.AskRoute || a == order.AskNearest {
				c.asks = append(c.asks, cursor.IDs[i])
			}
		}
	}
	if len(c.asks) == 0 || len(c.routers) == 0 {
		return
	}
	c.d = d
	c.next.Store(0)
	lanes := min(len(c.routers), len(c.asks), c.pool.Size())
	if c.pool == nil {
		c.route(0, 0)
	} else {
		b := c.pool.Begin(c.task, nil)
		b.Add(lanes)
		b.Wait()
	}
	c.start = (c.start + 1) % len(c.routers)
}

// route answers asks on goroutine w through the i-th router taken, till none is left
func (c *Coordinator) route(w, i int) {
	r := &c.routers[(c.start+i)%len(c.routers)]
	if c.read != nil {
		r.finder.Read(c.read(w))
	}
	q := c.forks[w]
	for {
		k := int(c.next.Add(1) - 1)
		if k >= len(c.asks) {
			return
		}
		id := c.asks[k]
		if !q.Seek(id) {
			continue
		}
		cur := q.Cursor()
		o := c.order.At(cur)
		at := c.cell.At(cur).Cell
		m := bodies.Member{ID: id, Cell: at, From: at, Leg: o.Leg, Domain: unit.DomainAt(nil, 0),
			Order: o}
		if b := c.base.At(cur); b != nil {
			m.Pos, m.Vel, m.Facing = b.Pos, b.Vel.Delta(), b.Vel.Dir
		}
		if z := c.z.At(cur); z != nil {
			m.Z = *z
		}
		if mv := c.mover.At(cur); mv != nil {
			m.Domain, m.Lift = mv.Domain, mv.Lift
		}
		answer(r.keep, m, o, c.d)
	}
}

func answer(keep spacing.Keeping, m bodies.Member, o *order.MoveOrder, d time.Duration) {
	if o.Ask == order.AskRoute {
		if path, found := keep.Route(m, m.From, o); found {
			o.Path, o.Waited, o.Ask = path, 0, order.AskNone
			return
		}
		if o.Waited += d; o.Waited < spacing.TargetWaitTimeout {
			o.Ask = order.NoRoute
			return
		}
	}
	dest, path, _, ok := keep.Lost(m, m.From, o, o.Waited)
	if !ok {
		o.Ask = order.NoPlace
		return
	}
	o.Target, o.Path, o.Waited, o.Ask = dest, path, 0, order.AskNone
	order.Found(o)
}
