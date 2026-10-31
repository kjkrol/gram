package routing

import (
	"cmp"
	"log"
	"math"
	"slices"
	"sync/atomic"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
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
	lanes   int // the routers the asks are shared among, the others idle
	d       time.Duration
	task    func(w, i int) // the method value, bound once

	fields  spacing.Fields
	fielded bool // some party asks this Update, its field expected

	hierarchies spacing.Hierarchies
	side        int     // of the hierarchies' clusters, in cells; 0, none kept
	from        float64 // how many cells apart a route's ends lie at least for it to be climbed
	// by ask, whether it is for a far route or the nearest free cell, its unit given the
	// hierarchy serving its class
	far []bool

	ranks     []ranked // the formations' members an Update lays out, in their order once sorted
	forms     []formed
	unordered bool // a formation marching unordered was said in the log

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

// Router adds a router: keep, searching through f; the first's f has the fields filled and the
// hierarchies built as it searches.
func (c *Coordinator) Router(f *spacing.Finder, keep spacing.Keeping) *Coordinator {
	if len(c.routers) == 0 {
		c.fields.Bind(f)
		c.hierarchies.Bind(f, c.side, c.from)
	}
	c.routers = append(c.routers, router{finder: f, keep: keep})
	return c
}

// Hierarchy has a route whose ends lie at least from cells apart climbed through the hierarchy
// serving its unit's class, of clusters side cells a side; side 0, every route searched as
// before. Call before the first Update.
func (c *Coordinator) Hierarchy(side int, from float64) *Coordinator {
	c.side, c.from = side, from
	if len(c.routers) > 0 {
		c.hierarchies.Bind(c.routers[0].finder, side, from)
	}
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

// Climbs counts the routes the routers climbed through hierarchies since they were made.
func (c *Coordinator) Climbs() uint64 {
	var n uint64
	for i := range c.routers {
		n += c.routers[i].finder.Climbs()
	}
	return n
}

// Building says a hierarchy kept has clusters left to stale or build.
func (c *Coordinator) Building() bool { return c.hierarchies.Due() }

// Builds counts the hierarchies' clusters the routers built since they were made.
func (c *Coordinator) Builds() uint64 {
	var n uint64
	for i := range c.routers {
		n += c.routers[i].finder.Builds()
	}
	return n
}

// Expansions counts the nodes the routers' searches expanded since they were made.
func (c *Coordinator) Expansions() uint64 {
	var n uint64
	for i := range c.routers {
		n += c.routers[i].finder.Expansions()
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
	c.task = c.route
}

func (c *Coordinator) Update(_ *goke.CmdBuf, d time.Duration) {
	c.asks, c.parties, c.far = c.asks[:0], c.parties[:0], c.far[:0]
	partied, climbing := false, false
	for c.query.All(); c.query.Next(); {
		cursor := c.query.Cursor()
		orders, cells := c.order.Slice(cursor), c.cell.Slice(cursor)
		for i := range orders {
			a := orders[i].Ask
			if a != order.AskRoute && a != order.AskNearest {
				continue
			}
			party := a == order.AskRoute && byField(&orders[i])
			// a party goes by its field, a walk that fails searched
			far := !byField(&orders[i]) && (a == order.AskNearest && c.hierarchies.On() ||
				c.hierarchies.Far(cells[i].Cell, orders[i].Target))
			c.asks, c.parties = append(c.asks, cursor.IDs[i]), append(c.parties, party)
			c.far = append(c.far, far)
			partied, climbing = partied || party, climbing || far
		}
	}
	c.climb(climbing)
	if len(c.asks) == 0 || len(c.routers) == 0 {
		return
	}
	c.fielded = partied && c.expect()
	c.d = d
	c.next.Store(0)
	c.lanes = min(len(c.routers), len(c.asks), c.pool.Size())
	if c.pool == nil {
		c.route(0, 0)
	} else {
		b := c.pool.Begin(c.task, nil)
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

// climb has the hierarchy serving the class of every unit asking far expected, where expect says
// one does, then the hierarchies staled and built through the first router, reading the board as
// the caller's goroutine does, before any ask is answered; an Update with nothing to stale or
// build and none expected reads nothing
func (c *Coordinator) climb(expect bool) {
	if !c.hierarchies.On() {
		return
	}
	for k := 0; expect && k < len(c.asks); k++ {
		if !c.far[k] || !c.query.Seek(c.asks[k]) {
			continue
		}
		m, _ := c.member(c.query, c.asks[k])
		if key, ok := c.hierarchies.Key(m); ok {
			c.hierarchies.Expect(key)
		}
	}
	due := c.hierarchies.Due()
	if !due && !expect {
		return
	}
	if due && c.read != nil {
		c.routers[0].finder.Read(c.read(0))
	}
	c.hierarchies.Sweep()
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
		if key, ok := c.hierarchies.Key(m); ok && c.far[k] {
			m.Hierarchy = c.hierarchies.Take(key)
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
		m.Domain, m.Lift, m.Weight = mv.Domain, mv.Lift, mv.Weight
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

// a member of a formation as an Update lays it out
type ranked struct {
	id    uid.UID64
	order *order.MoveOrder
	at    geom.Vec
	z     world.Z
	form  int32 // its formation among the Update's
	cost  float32
	per   float32  // what a world unit of the way to the goal costs where it stands
	laid  geom.Vec // a point of the cell its footprint is laid from, where in it it stands
}

// a formation as an Update lays it out: its group, the first member met, whose key takes the
// field, pending till every member has seeded it, and same, the formation it is where another
// run of its members met it first; was, its head the Update before, waiting or not, with its
// gap's narrowest and its patience
type formed struct {
	group   uint32
	first   uid.UID64
	field   *pathfind.Field
	pending bool
	same    int32
	was     headWas
}

type headWas struct {
	id        uid.UID64
	met       bool
	waited    bool
	narrowest float32
	patience  time.Duration
}

// Form lays out the formations the units ids march in, before they move, as the fields of their
// goals lead them: each member's Place, AheadOf and Lag, the head's Gap and Closing. Made without
// a board's read, it lays out none.
func (c *Coordinator) Form(ids []uid.UID64) {
	if c.read == nil || len(c.routers) == 0 || len(ids) == 0 {
		return
	}
	brd := c.read(0).Board()
	span := brd.CellSpan()
	c.ranks, c.forms = c.ranks[:0], c.forms[:0]
	c.fields.Sweep()
	// no group's: a key no member's is
	last, _ := c.fields.Key(bodies.Member{}, &order.MoveOrder{})
	for _, id := range ids {
		if !c.query.Seek(id) {
			continue
		}
		m, o := c.member(c.query, id)
		was := headWas{id: id, met: o.Place == 1, waited: o.Kept, narrowest: o.Narrowest,
			patience: o.Patience}
		o.Place, o.AheadOf, o.Lag, o.Gap, o.HeadWaits = 0, 0, 0, 0, false
		if o.Party < 2 {
			continue
		}
		q, ok := c.fields.Key(m, o)
		if !ok {
			if o.At != (geom.Vec{}) && !o.GivingWay {
				c.unorder()
			}
			continue
		}
		if n := len(c.forms); n == 0 || q != last || c.forms[n-1].group != o.Group {
			f := formed{group: o.Group, first: id, same: int32(n)}
			field, build := c.fields.Take(q)
			switch {
			case build: // another run of the formation seeded it: filled before any other takes
				c.fill(field)
			case field == nil:
				f.pending = true
			default: // kept, and lately asked for
				c.fields.Expect(q, m.Cell)
			}
			f.field, last = field, q
			c.forms = append(c.forms, f)
		}
		if f := &c.forms[len(c.forms)-1]; was.met {
			f.was = was
		}
		if c.forms[len(c.forms)-1].pending {
			c.fields.Expect(q, m.Cell)
		}
		r := ranked{id: id, order: o, at: m.Centre(), z: m.Z, form: int32(len(c.forms) - 1)}
		if r.laid = r.at; max(m.Pos.Size.X, m.Pos.Size.Y) > span {
			r.laid = unit.LaidFrom(brd, m.Pos.Size, r.at)
		}
		c.ranks = append(c.ranks, r)
	}
	c.takeFields()
	for i := range c.ranks {
		r := &c.ranks[i]
		f := c.forms[r.form]
		r.form, r.cost, r.per = f.same, f32.Inf(1), 0
		if f.field == nil {
			continue
		}
		at, deck, ok := spacing.LevelOf(brd, brd, r.laid, &r.z)
		if !ok {
			continue
		}
		level := pathfind.Ground
		if deck {
			level = pathfind.Deck
		}
		if cost, on := f.field.Cost(at, level); on {
			r.cost, r.per = costAt(brd, f.field, at, level, cost, r.laid)
		}
	}
	slices.SortFunc(c.ranks, func(a, b ranked) int {
		return cmp.Or(cmp.Compare(a.form, b.form), cmp.Compare(a.cost, b.cost),
			cmp.Compare(a.id, b.id))
	})
	for start := 0; start < len(c.ranks); {
		end := start + 1
		for end < len(c.ranks) && c.ranks[end].form == c.ranks[start].form {
			end++
		}
		c.lay(c.ranks[start:end])
		start = end
	}
}

// takeFields has the fields of the formations pending filled, their members' cells the seeds; a
// formation met in two runs is one
func (c *Coordinator) takeFields() {
	for i := range c.forms {
		f := &c.forms[i]
		if f.pending && c.query.Seek(f.first) {
			m, o := c.member(c.query, f.first)
			q, _ := c.fields.Key(m, o)
			field, build := c.fields.Take(q)
			if build {
				c.fill(field)
			}
			f.field = field
		}
		for j := range i {
			if c.forms[j].group == f.group && c.forms[j].field == f.field {
				f.same = int32(j)
				if f.was.met {
					c.forms[j].was = f.was
				}
				break
			}
		}
	}
}

// fill fills field, taken to build, through the first router on the caller's goroutine
func (c *Coordinator) fill(field *pathfind.Field) {
	r := &c.routers[0]
	r.finder.Read(c.read(0))
	c.fields.Fill(field, r.finder)
	c.fields.Give(field)
}

// lay writes members, one formation's in its order, on their orders: each one's place, whom it
// walks behind and how far, whether the head waited the step before, the widest gap on the head,
// which a new head takes on with the old one's narrowest and patience; one none of whose members
// stands on its field marches unordered
func (c *Coordinator) lay(members []ranked) {
	f := &c.forms[members[0].form]
	if f32.IsInf(members[0].cost, 1) {
		if f.field != nil {
			c.unorder()
		}
		return
	}
	head := members[0].order
	switch {
	case !f.was.met:
		head.Narrowest = 0
	case f.was.id != members[0].id:
		head.Narrowest, head.Patience = f.was.narrowest, f.was.patience
	}
	for k := range members {
		r := &members[k]
		r.order.Place, r.order.HeadWaits = uint16(min(k+1, math.MaxUint16)), f.was.waited
		if k == 0 {
			continue
		}
		ahead := &members[k-1]
		r.order.AheadOf, r.order.Lag = ahead.id, f32.Inf(1)
		if f32.IsInf(r.cost, 1) {
			continue
		}
		// read at the dearer ground of the two it errs on the near side where the ground changes,
		// and never past the straight way between them
		lag := (r.cost - ahead.cost) / max(r.per, ahead.per)
		lag = min(lag, f32.Hypot(r.at.X-ahead.at.X, r.at.Y-ahead.at.Y))
		if r.order.Lag = lag; lag > head.Gap {
			head.Gap = lag
		}
	}
}

func (c *Coordinator) unorder() {
	if !c.unordered {
		log.Printf("navigation: a formation marches unordered, as a MoveTo: no field of its goal " +
			"is kept, or none reaches it")
		c.unordered = true
	}
}

// costAt is the cost of the way to f's goal from p, in c on level l, c's cost being cost, and
// what a world unit of it costs there: between c's middle and the next cell's along the field's
// step out of c, behind c's middle along the step from the neighbour that leads into c, so it
// runs on unbroken from cell to cell; else as at full pace
func costAt(g grid.Grid, f *pathfind.Field, c cell.ID, l pathfind.Level, cost float32,
	p geom.Vec) (at, per float32) {
	mid := g.CellCenter(c)
	d := geom.NewVec(p.X-mid.X, p.Y-mid.Y)
	t, drop, span := float32(-1), float32(0), float32(0)
	if n, nl, ok := f.Next(c, l); ok {
		t, drop, span = onStep(g, f, c, n, nl, cost, d)
	}
	if t >= 0 {
		return cost - min(t, 1)*drop, drop / span
	}
	if at, per, ok := behind(g, f, c, cost, d); ok {
		return at, per
	}
	if span > 0 {
		return cost - max(t, -1)*drop, drop / span
	}
	step := g.Step(0)
	unit := f32.Hypot(step.X, step.Y)
	return cost + min(f32.Hypot(d.X, d.Y), unit)/unit, 1 / unit
}

// behind is costAt for d, off c's middle, on the step into c from the neighbour d points to,
// where the field leads that one into c; false where it does not
func behind(g grid.Grid, f *pathfind.Field, c cell.ID, cost float32,
	d geom.Vec) (at, per float32, ok bool) {
	i := grid.Nearest(g, d)
	u, ok := g.Toward(c, i)
	if !ok {
		return 0, 0, false
	}
	if into, _, ok := f.Next(u, pathfind.Either); !ok || into != c {
		return 0, 0, false
	}
	s := g.Step(i)
	back, _ := f.Cost(u, pathfind.Either)
	ss := s.X*s.X + s.Y*s.Y
	rise := back - cost
	if rise <= 0 || ss <= 0 {
		return 0, 0, false
	}
	return cost + min((d.X*s.X+d.Y*s.Y)/ss, 1)*rise, rise / f32.Sqrt(ss), true
}

// onStep is where d, off c's middle, lies along the step from c to n, a share of it, what the
// step costs and how long it is; a share of -1 where the step is none
func onStep(g grid.Grid, f *pathfind.Field, c, n cell.ID, nl pathfind.Level, cost float32,
	d geom.Vec) (t, drop, span float32) {
	i, way := grid.Way(g, c, n)
	if !way {
		return -1, 0, 0
	}
	s := g.Step(i)
	next, _ := f.Cost(n, nl)
	ss := s.X*s.X + s.Y*s.Y
	if drop = cost - next; drop <= 0 || ss <= 0 {
		return -1, 0, 0
	}
	return (d.X*s.X + d.Y*s.Y) / ss, drop, f32.Sqrt(ss)
}
