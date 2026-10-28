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
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Coordinator answers the orders' asks (MoveOrder.Ask): a route to the Target, or the nearest
// free cell round it; an Update of d for a unit that has waited d more. The asks are shared
// among the pool's goroutines, each searching through one of the routers at a time. The routes
// of a party, fieldParty or more units under one MoveTo, are read off one field of the way to
// its goal.
type Coordinator struct {
	pool    *parallel.Pool
	read    func(w int) *board.Read // nil: the routers read what they were made over
	routers []router
	start   int         // the router the first goroutine takes, one on every Update
	asks    []uid.UID64 // the units that asked
	groups  []uint32    // by ask, the group of an ask for a route
	next    atomic.Int64
	lanes   int          // the routers the asks are shared among, the others idle
	idled   atomic.Int64 // the idle routers taken
	d       time.Duration
	task    func(w, i int) // the method values, bound once
	idle    func(w int)

	fields  spacing.Fields
	parties map[uint32]uint16 // by group, the orders under way, counted where a group asks
	fielded bool              // some party asks this Update, its field expected

	query *goke.Query
	forks []*goke.Query // by goroutine, query the first
	cell  goke.Comp[unit.At]
	order goke.Comp[order.MoveOrder]
	mover goke.OptComp[unit.Mover]
	base  goke.OptComp[world.Base]
	z     goke.OptComp[world.Z]
}

// fieldParty is how many orders under way of one MoveTo have its asks read their routes off a
// field
const fieldParty = 4

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

// Router adds a router: keep, searching through f; the first's f has the fields filled as it
// searches.
func (c *Coordinator) Router(f *spacing.Finder, keep spacing.Keeping) *Coordinator {
	if len(c.routers) == 0 {
		c.fields.Bind(f)
	}
	c.routers = append(c.routers, router{finder: f, keep: keep})
	return c
}

// Fills counts the fields the routers filled since they were made.
func (c *Coordinator) Fills() uint64 {
	var n uint64
	for i := range c.routers {
		n += c.routers[i].finder.Fills()
	}
	return n
}

// Searches counts the routes the routers searched for since they were made: of a walk off a
// field, its last leg alone.
func (c *Coordinator) Searches() uint64 {
	var n uint64
	for i := range c.routers {
		n += c.routers[i].finder.Searches()
	}
	return n
}

func (c *Coordinator) Init(si *goke.SysInit) {
	c.query = si.NewQueryBuilder(&c.cell, &c.order).Optional(&c.mover).Optional(&c.base).
		Optional(&c.z).Build()
	c.forks = append(c.forks[:0], c.query)
	for range c.pool.Size() - 1 {
		c.forks = append(c.forks, c.query.Fork())
	}
	c.task, c.idle = c.route, c.wipe
	c.parties = map[uint32]uint16{}
}

func (c *Coordinator) Update(_ *goke.CmdBuf, d time.Duration) {
	c.asks, c.groups = c.asks[:0], c.groups[:0]
	grouped := false
	for c.query.All(); c.query.Next(); {
		cursor := c.query.Cursor()
		orders := c.order.Slice(cursor)
		for i := range orders {
			a := orders[i].Ask
			if a != order.AskRoute && a != order.AskNearest {
				continue
			}
			var group uint32
			if a == order.AskRoute {
				group = orders[i].Group
			}
			c.asks, c.groups = append(c.asks, cursor.IDs[i]), append(c.groups, group)
			grouped = grouped || group != 0
		}
	}
	if len(c.asks) == 0 || len(c.routers) == 0 {
		return
	}
	c.fielded = grouped && c.expect()
	c.d = d
	c.next.Store(0)
	c.lanes = min(len(c.routers), len(c.asks), c.pool.Size())
	c.idled.Store(int64(c.lanes))
	if c.pool == nil {
		c.route(0, 0)
		c.wipe(0)
	} else {
		b := c.pool.Begin(c.task, c.idle)
		b.Add(c.lanes)
		b.Wait()
	}
	c.start = (c.start + 1) % len(c.routers)
}

// expect counts the orders under way of each group and has the field of every party asking for
// routes expected, from the cells its asks stand in; false where no party asks
func (c *Coordinator) expect() bool {
	clear(c.parties)
	for c.query.All(); c.query.Next(); {
		orders := c.order.Slice(c.query.Cursor())
		for i := range orders {
			if g := orders[i].Group; g != 0 {
				c.parties[g]++
			}
		}
	}
	swept := false
	for k, id := range c.asks {
		if c.parties[c.groups[k]] < fieldParty || !c.query.Seek(id) {
			continue
		}
		m, o := c.member(c.query, id)
		q, ok := c.fields.Key(m, o)
		if !ok {
			continue
		}
		if !swept {
			c.fields.Sweep()
			swept = true
		}
		c.fields.Expect(q, m.Cell)
	}
	return swept
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
		m, o := c.member(q, id)
		if c.fielded && c.parties[c.groups[k]] >= fieldParty {
			m.Field = c.field(r, m, o)
		}
		answer(r.keep, m, o, c.d)
	}
}

// member is the unit q's cursor stands at, id, as it asks, and its order
func (c *Coordinator) member(q *goke.Query, id uid.UID64) (bodies.Member, *order.MoveOrder) {
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
	return m, o
}

// field is the field of m's group's goal, filled through r by the first ask to take it
func (c *Coordinator) field(r *router, m bodies.Member, o *order.MoveOrder) *pathfind.Field {
	q, ok := c.fields.Key(m, o)
	if !ok {
		return nil
	}
	f, build := c.fields.Take(q)
	if build {
		c.fields.Fill(f, r.finder)
		c.fields.Give(f)
	}
	return f
}

// wipeWithin is how few searches a router is left before its stamps wrap when an idle goroutine
// wipes them
const wipeWithin = 64

// wipe wipes the stamps of the routers idle this Update near their wrap, on w with nothing to do
func (c *Coordinator) wipe(int) {
	for {
		j := int(c.idled.Add(1) - 1)
		if j >= len(c.routers) {
			return
		}
		c.routers[(c.start+j)%len(c.routers)].finder.Wipe(wipeWithin)
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
