package spacing

import (
	"cmp"
	"slices"
	"sync"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/uid"
)

// Keeping is how units keep out of each other's way under a Spacing: their orders, their steps
// and what they do on meeting.
type Keeping interface {
	Occupancy() cell.Occupancy
	Begin(gather func([]bodies.Body) []bodies.Body)
	// an appended goal is queued on the member's own order, not issued
	Orders(members []bodies.Member, cmd order.MoveTo, issue func(bodies.Member, order.MoveOrder))
	Look(m bodies.Member, at geom.Vec) order.MoveOrder
	// Ready may move o's Target and Spot
	Ready(m bodies.Member, o *order.MoveOrder)
	Route(m bodies.Member, from cell.ID, o *order.MoveOrder) (order.Path, bool)
	// Lost: wait on, or settle on dest along path, or with neither give up
	Lost(m bodies.Member, from cell.ID, o *order.MoveOrder, waited time.Duration) (dest cell.ID, path order.Path, wait, ok bool)
	// a zero dir keeps the heading m has
	Steer(m bodies.Member, st steering.Helm, dir geom.Vec, speed float32)
	Watch(m bodies.Member, o *order.MoveOrder, toward cell.ID, want geom.Vec, d time.Duration)
	Bump(m bodies.Member, o *order.MoveOrder) BumpAnswer
	Blocked(m bodies.Member, o *order.MoveOrder, c cell.ID, deck bool,
		d time.Duration) (holder uid.UID64, known bool)
	Within(m bodies.Member) float32
	On(m bodies.Member, c cell.ID, level order.Level) bool
	// near is the caller's scratch for the bodies near, nil for the keeping's own
	MayStep(m bodies.Member, ahead cell.ID, deck bool, heading geom.Vec, near *bodies.Nearby) bool
	// true: by their boxes striking; false: by a step into a held cell refused
	ByContact() bool

	Other(id uid.UID64) (bodies.Body, bool)
	OnGoal(self, other bodies.Body) bool
	StepAside(m bodies.Member, of bodies.Body) (order.MoveOrder, bool)
	Detour(m bodies.Member, o *order.MoveOrder, other bodies.Body) BumpAnswer
	Pass(m bodies.Member, o *order.MoveOrder, other bodies.Body)
	Settle(m bodies.Member, o *order.MoveOrder, other bodies.Body) bool
}

// YieldShare is the least of the pace the ground gives a unit on the level that a step aside
// may leave it by its slope: a flat forest is stepped into, a cliff is not.
const YieldShare = 1.0 / 3

type holders interface {
	Holder(c cell.ID, deck bool, d cell.Domain) (uid.UID64, bool)
}

// BumpAnswer is what a unit does after it struck someone: stop and plan, carry on or give up.
type BumpAnswer uint8

const (
	StopAndPlan BumpAnswer = iota
	CarryOn
	GiveUp
)

// the cells a route goes round; blocked asks about them, the method value bound once
type avoiding struct {
	target  cell.ID
	known   [4]cell.ID
	n       uint8
	blocked func(cell.ID) bool
	kept    *Answers
}

// a search blind to the others reads nothing but the board, so over a board counting its
// changes its answer is kept, in kept, till the next one
func (a *avoiding) bind(f *Finder, kept *Answers) {
	a.blocked, a.kept = a.avoids, kept
	if _, blind := f.Occupancy.(OpenOccupancy); blind && kept.board == nil {
		kept.board, _ = f.Terrain.(interface{ Version() uint64 })
	}
}

func (a *avoiding) avoids(c cell.ID) bool {
	return c != a.target && slices.Contains(a.known[:a.n], c)
}

// what a search for a route reads besides the board: round, the cells it goes round, sorted,
// the target left out
type asked struct {
	from, to cell.ID
	end      order.Level
	domain   cell.Domain
	body     pathfind.Body
	round    [4]cell.ID
	n        uint8
	around   bool
}

type answer struct {
	path order.Path
	ok   bool
	done bool // false: being worked out
}

// Answers are the searches' answers given since the board last changed, at most keptAnswers,
// all let go at once. Keepings on several goroutines may share them: one asking what another
// works out waits for its answer. The zero value is ready.
type Answers struct {
	mu      sync.Mutex
	worked  sync.Cond                     // an answer worked out
	board   interface{ Version() uint64 } // nil: none kept
	version uint64
	index   map[asked]int32
	given   []answer
	working int // the answers being worked out
}

const keptAnswers = 4096

// a route m's group's field gives is walked off it, never kept; a search asked again over the
// board as it stands gets the answer it gave, unrun
func (a *avoiding) find(f *Finder, m bodies.Member, from cell.ID, o *order.MoveOrder,
	around bool) (order.Path, bool) {
	if !around && m.Field != nil {
		if p, ok := f.walkField(m.Field, m, from, o); ok {
			return p, true
		}
	}
	q := asked{from: from, to: o.Target, end: o.Level, domain: m.Domain, body: m.Body(),
		around: around}
	if around {
		for _, c := range o.Avoid[:o.Avoids] {
			if c != o.Target {
				q.round[q.n] = c
				q.n++
			}
		}
		slices.Sort(q.round[:q.n])
	}
	slot, p, ok, known := a.kept.take(q)
	if known {
		return p, ok
	}
	if around {
		a.target, a.known, a.n = o.Target, o.Avoid, o.Avoids
		p, ok = f.findPathAround(m.ID, m.Domain, q.body, from, o.Target, o.Level, a.blocked)
	} else {
		p, ok = f.FindPath(m.ID, m.Domain, q.body, from, o.Target, o.Level)
	}
	a.kept.give(slot, p, ok)
	return p, ok
}

// take is q's answer, known where it is kept; else the slot of q's answer, which the caller
// works out and gives, -1 where none is kept
func (k *Answers) take(q asked) (slot int32, p order.Path, ok, known bool) {
	if k.board == nil {
		return -1, p, false, false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if v := k.board.Version(); k.working == 0 && (v != k.version || len(k.given) >= keptAnswers) {
		k.version, k.given = v, k.given[:0]
		clear(k.index)
	}
	for {
		i, seen := k.index[q]
		if !seen {
			break
		}
		if g := &k.given[i]; g.done {
			return 0, g.path, g.ok, true
		}
		if k.worked.L == nil {
			k.worked.L = &k.mu
		}
		k.worked.Wait()
	}
	if k.index == nil {
		k.index = map[asked]int32{}
	}
	k.index[q] = int32(len(k.given))
	k.given = append(k.given, answer{})
	k.working++
	return int32(len(k.given) - 1), p, false, false
}

func (k *Answers) give(slot int32, p order.Path, ok bool) {
	if slot < 0 {
		return
	}
	k.mu.Lock()
	k.given[slot] = answer{path: p, ok: ok, done: true}
	k.working--
	k.mu.Unlock()
	k.worked.Broadcast()
}

// round the cells learned unless that found nothing within o's Retry, else any route; the first
// of the two run says whether o waits
func (a *avoiding) route(f *Finder, m bodies.Member, from cell.ID, o *order.MoveOrder) (order.Path,
	bool) {
	due := o.Retry == 0
	if o.Avoids > 0 && due {
		if p, ok := a.find(f, m, from, o, true); ok {
			order.Found(o)
			return p, true
		}
		order.Failed(o)
	}
	p, ok := a.find(f, m, from, o, false)
	switch {
	case o.Avoids > 0:
	case ok:
		order.Found(o)
	case due:
		order.Failed(o)
	}
	return p, ok
}

// CellKeeping gives a unit a cell to itself in each domain, as the occupancy says (CellSpacing).
type CellKeeping struct {
	finder *Finder
	occ    cell.Occupancy
	who    holders // nil: nobody is asked off a cell
	index  bodies.Index
	avoid  avoiding
	// whom held answers for, through the method value bound once
	holder   uid.UID64
	holderIn cell.Domain
	heldBy   func(c cell.ID, deck bool) bool
	asides   [8]scored
}

var _ Keeping = (*CellKeeping)(nil)

// NewCellKeeping keeps the units in cells of their own over finder's board and occ.
func NewCellKeeping(finder *Finder, occ cell.Occupancy) *CellKeeping {
	k := &CellKeeping{finder: finder, occ: occ, index: bodies.NewIndex(finder.grid)}
	k.who, _ = occ.(holders)
	k.avoid.bind(finder, &Answers{})
	k.heldBy = k.isHeld
	return k
}

// WithAnswers has k keep its searches' answers in a, which other keepings may share.
func (k *CellKeeping) WithAnswers(a *Answers) *CellKeeping {
	k.avoid.bind(k.finder, a)
	return k
}

func (k *CellKeeping) Occupancy() cell.Occupancy { return k.occ }

func (k *CellKeeping) Begin(gather func([]bodies.Body) []bodies.Body) {
	k.index.Build(gather(k.index.Bodies[:0]))
}

func (k *CellKeeping) Orders(members []bodies.Member, cmd order.MoveTo, issue func(bodies.Member,
	order.MoveOrder)) {
	target, at := cmd.Cell, cmd.At
	if at == (geom.Vec{}) {
		at = k.finder.grid.CellCenter(target)
	}
	var moves []bodies.Member
	for _, m := range members {
		if cmd.Append && m.Order != nil {
			m.Order.Enqueue(order.Goal{Cell: target, At: at, Level: cmd.Level})
			continue
		}
		standing := m.Order == nil || !m.Order.Leg.Active
		if !cmd.Append && standing && m.Cell == target && cmd.At != (geom.Vec{}) &&
			k.finder.on(m, target, cmd.Level) {
			issue(m, order.MoveOrder{Target: target, Face: cmd.At})
			continue
		}
		moves = append(moves, m)
	}
	pf := k.finder
	slices.SortStableFunc(moves, func(a, b bodies.Member) int {
		return cmp.Compare(pf.grid.Distance(a.From, target), pf.grid.Distance(b.From, target))
	})
	if len(moves) == 0 {
		return
	}
	var dom cell.Domain
	for _, m := range moves {
		dom |= m.Domain
	}
	waiting := moves
	for c := range pf.rings(target, dom, moves[0].Domain) {
		kind := pf.Terrain.Kind(c)
		i := slices.IndexFunc(waiting, func(m bodies.Member) bool { return kind.Admits(m.Domain) })
		if i < 0 {
			continue
		}
		issue(waiting[i], order.MoveOrder{Target: c, Level: cmd.Level, At: at, Leg: waiting[i].Leg})
		if i == 0 {
			waiting = waiting[1:]
		} else {
			waiting = slices.Delete(waiting, i, i+1)
		}
		if len(waiting) == 0 {
			break
		}
	}
}

func (k *CellKeeping) Look(m bodies.Member, at geom.Vec) order.MoveOrder {
	order := order.MoveOrder{Target: m.Cell, Face: at}
	if m.Leg.Active {
		order.Target, order.Leg = m.Leg.To, m.Leg
	}
	return order
}

func (k *CellKeeping) Ready(bodies.Member, *order.MoveOrder) {}

func (k *CellKeeping) Route(m bodies.Member, from cell.ID, o *order.MoveOrder) (order.Path, bool) {
	return k.avoid.route(k.finder, m, from, o)
}

func (k *CellKeeping) held(m bodies.Member) func(c cell.ID, deck bool) bool {
	k.holder, k.holderIn = m.ID, m.Domain
	return k.heldBy
}

func (k *CellKeeping) isHeld(c cell.ID, deck bool) bool {
	return !k.occ.CanEnter(c, deck, k.holder, k.holderIn)
}

func (k *CellKeeping) Lost(m bodies.Member, from cell.ID, o *order.MoveOrder, waited time.Duration) (cell.ID, order.Path, bool, bool) {
	if waited < TargetWaitTimeout {
		return 0, order.Path{}, true, false
	}
	dest, path, ok := k.finder.nearestFree(m.ID, m.Domain, m.Body(), from, o.Target, o.Level,
		k.held(m))
	return dest, path, false, ok
}

func (k *CellKeeping) On(m bodies.Member, c cell.ID, level order.Level) bool {
	return k.finder.on(m, c, level)
}

func (k *CellKeeping) Steer(_ bodies.Member, st steering.Helm, dir geom.Vec, speed float32) {
	if dir != (geom.Vec{}) {
		st.Request(dir)
	}
	st.RequestSpeed(speed)
}

func (k *CellKeeping) Watch(m bodies.Member, o *order.MoveOrder, _ cell.ID, _ geom.Vec,
	_ time.Duration) {
	o.Stalled, o.Trailed = 0, 0
	if m.Cell == o.Toward {
		o.Stalls = 0
	}
}

// Held after stallAfter is the last word against rules that never move the unit; a wait on one
// it trails, while they are under way, is no stall
func (k *CellKeeping) Blocked(m bodies.Member, o *order.MoveOrder, c cell.ID, deck bool,
	d time.Duration) (uid.UID64, bool) {
	if o.Toward != c {
		o.Toward, o.Stalled, o.Trailed = c, 0, 0
	}
	var holder uid.UID64
	known := false
	if k.who != nil {
		holder, known = k.who.Holder(c, deck, m.Domain)
	}
	if trailing(&k.index, o) {
		o.Trailed += d
		return holder, known
	}
	if o.Stalled += d; o.Stalled >= StallAfter {
		o.Bumped, o.Held, o.Hit, o.HitUnit, o.Stalled = true, true, holder, known, 0
	}
	return holder, known
}

func (k *CellKeeping) Bump(m bodies.Member, o *order.MoveOrder) BumpAnswer {
	if !o.Held {
		return StopAndPlan
	}
	o.Held, o.Hit, o.HitUnit = false, 0, false
	if o.Stalls++; o.Stalls > maxStalls {
		return GiveUp
	}
	if o.Toward == o.Target {
		dest, path, ok := k.finder.nearestFree(m.ID, m.Domain, m.Body(), m.From, o.Target, o.Level,
			k.held(m))
		if !ok {
			return GiveUp
		}
		o.Target, o.Path, o.Leg = dest, path, order.Leg{}
		order.Found(o)
		return CarryOn
	}
	order.Learn(o, o.Toward)
	o.Path, o.Leg = order.Path{}, order.Leg{}
	return CarryOn
}

func (k *CellKeeping) Within(bodies.Member) float32 { return ArrivalEpsilon }

func (k *CellKeeping) MayStep(m bodies.Member, ahead cell.ID, deck bool, _ geom.Vec,
	_ *bodies.Nearby) bool {
	return ahead == m.Cell || k.occ.CanEnter(ahead, deck, m.ID, m.Domain)
}

func (k *CellKeeping) Other(id uid.UID64) (bodies.Body, bool) { return k.index.Of(id) }

type scored struct {
	c     cell.ID
	score float32
}

// across the comer's way first, then behind, a slantwise step last; never ahead of it. Whether m
// may step there is the caller's to ask, in this order.
func (k *CellKeeping) aside(m bodies.Member, from cell.ID, way geom.Vec) []scored {
	grid := k.finder.grid
	home := m.Cell
	hc := grid.CellCenter(home)
	out := k.asides[:0]
	for i := range grid.Directions() {
		n, ok := grid.Toward(home, i)
		if !ok || n == from {
			continue
		}
		c := grid.CellCenter(n)
		d := geom.NewVec(c.X-hc.X, c.Y-hc.Y)
		l := f32.Hypot(d.X, d.Y)
		if l == 0 {
			continue
		}
		dot := (d.X*way.X + d.Y*way.Y) / l
		if dot > 0.3 {
			continue
		}
		score := f32.Abs(dot)
		if _, _, diagonal := grid.DiagonalNeighbors(home, n); diagonal {
			score++
		}
		out = append(out, scored{n, score})
	}
	slices.SortStableFunc(out, func(a, b scored) int { return cmp.Compare(a.score, b.score) })
	return out
}

func (k *CellKeeping) wayFrom(m bodies.Member, asker bodies.Body) (cell.ID, geom.Vec) {
	grid := k.finder.grid
	from, _ := grid.CellAt(asker.At)
	a, b := grid.CellCenter(from), grid.CellCenter(m.Cell)
	d := geom.NewVec(b.X-a.X, b.Y-a.Y)
	if l := f32.Hypot(d.X, d.Y); l > 0 {
		d = geom.NewVec(d.X/l, d.Y/l)
	}
	return from, d
}

func (k *CellKeeping) ByContact() bool { return false }

func (k *CellKeeping) StepAside(m bodies.Member, of bodies.Body) (order.MoveOrder, bool) {
	from, way := k.wayFrom(m, of)
	terrain := k.finder.Terrain
	for _, s := range k.aside(m, from, way) {
		c := s.c
		if !terrain.Kind(c).Admits(m.Domain) {
			continue
		}
		if deck, ok := k.finder.stepsAside(m, c); !ok || !k.occ.CanEnter(c, deck, m.ID, m.Domain) {
			continue
		}
		o := order.MoveOrder{Target: c, GivingWay: true}
		if m.Facing != (geom.Vec{}) {
			c := m.Centre()
			o.Face = geom.NewVec(c.X+m.Facing.X*16, c.Y+m.Facing.Y*16)
		}
		return o, true
	}
	return order.MoveOrder{}, false
}

// nothing to go round once m steps on, nor round other giving way: it is leaving the cell
func (k *CellKeeping) Detour(m bodies.Member, o *order.MoveOrder, other bodies.Body) BumpAnswer {
	if o.Leg.Active || other.GivingWay {
		return CarryOn
	}
	c := o.Toward
	if c != o.Target && c != m.Cell {
		order.Learn(o, c)
	}
	if o.Cornered && o.Retry > 0 {
		o.Holding = 0
		return CarryOn
	}
	path, ok := k.Route(m, m.From, o)
	o.Holding, o.Cornered = 0, !ok || path.Index < path.Length && slices.Contains(path.Steps[path.Index:path.Length], c)
	if !o.Cornered {
		o.Path = path
	}
	return CarryOn
}

func (k *CellKeeping) Pass(bodies.Member, *order.MoveOrder, bodies.Body) {}

func (k *CellKeeping) OnGoal(self, other bodies.Body) bool { return other.Cell == self.Goal }

func (k *CellKeeping) Settle(m bodies.Member, o *order.MoveOrder, _ bodies.Body) bool {
	dest, path, ok := k.finder.nearestFree(m.ID, m.Domain, m.Body(), m.From, o.Target, o.Level,
		k.held(m))
	if !ok {
		return false
	}
	o.Target, o.Path, o.Leg = dest, path, order.Leg{}
	order.Found(o)
	return true
}
