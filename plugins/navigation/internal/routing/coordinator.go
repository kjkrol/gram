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
// among the pool's goroutines, each searching through one of the routers at a time.
type Coordinator struct {
	pool    *parallel.Pool
	read    func(w int) *board.Read // nil: the routers read what they were made over
	routers []router
	start   int         // the router the first goroutine takes, one on every Update
	asks    []uid.UID64 // the units that asked
	parties []bool      // by ask, whether a party's unit asks for its route
	next    atomic.Int64
	lanes   int          // the routers the asks are shared among, the others idle
	idled   atomic.Int64 // the idle routers taken
	d       time.Duration
	task    func(w, i int) // the method values, bound once
	idle    func(w int)

	fields  spacing.Fields
	fielded bool // some party asks this Update, its field expected

	query *goke.Query
	forks []*goke.Query // by goroutine, query the first
	cell  goke.Comp[unit.At]
	order goke.Comp[order.MoveOrder]
	mover goke.OptComp[unit.Mover]
	base  goke.OptComp[world.Base]
	z     goke.OptComp[world.Z]
}

// fieldParty is how many orders the MoveTo that gave one sent for its asks to read their routes
// off a field
const fieldParty = 4

// whether o's asks read their routes off its party's field: a MoveTo's party, or any marching in
// a formation, which the field lays out
func byField(o *order.MoveOrder) bool {
	return o.Party >= fieldParty || o.Formation.Shape != order.NoFormation && o.Party > 1
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

// Walks counts the routes the routers walked off fields since they were made.
func (c *Coordinator) Walks() uint64 {
	var n uint64
	for i := range c.routers {
		n += c.routers[i].finder.Walks()
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
}

func (c *Coordinator) Update(_ *goke.CmdBuf, d time.Duration) {
	c.asks, c.parties = c.asks[:0], c.parties[:0]
	partied := false
	for c.query.All(); c.query.Next(); {
		cursor := c.query.Cursor()
		orders := c.order.Slice(cursor)
		for i := range orders {
			a := orders[i].Ask
			if a != order.AskRoute && a != order.AskNearest {
				continue
			}
			party := a == order.AskRoute && byField(&orders[i])
			c.asks, c.parties = append(c.asks, cursor.IDs[i]), append(c.parties, party)
			partied = partied || party
		}
	}
	if len(c.asks) == 0 || len(c.routers) == 0 {
		return
	}
	c.fielded = partied && c.expect()
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

// expect has the field of every party asking for routes expected, from the cells its asks stand
// in; false where none has one
func (c *Coordinator) expect() bool {
	swept := false
	for k, id := range c.asks {
		if !c.parties[k] || !c.query.Seek(id) {
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
		if c.fielded && c.parties[k] {
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
