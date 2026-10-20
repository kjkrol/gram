package navigation

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type keeping interface {
	occupancy() cell.Occupancy
	begin(gather func([]body) []body)
	// an appended goal is queued on the member's own order, not issued
	orders(members []member, cmd MoveTo, issue func(member, MoveOrder))
	look(m member, at geom.Vec) MoveOrder
	// ready may move o's Target and Spot
	ready(m member, o *MoveOrder)
	route(m member, from cell.ID, o *MoveOrder) (Path, bool)
	// lost: wait on, or settle on dest along path, or with neither give up
	lost(m member, from cell.ID, o *MoveOrder, waited time.Duration) (dest cell.ID, path Path, wait, ok bool)
	// a zero dir keeps the heading m has
	steer(m member, st steering.Helm, dir geom.Vec, speed float64)
	watch(m member, o *MoveOrder, toward cell.ID, want geom.Vec, d time.Duration)
	bump(m member, o *MoveOrder) bumpAnswer
	blocked(m member, o *MoveOrder, c cell.ID, deck bool, d time.Duration) (holder uid.UID64,
		known bool)
	within(m member) float64
	on(m member, c cell.ID, level Level) bool
	mayStep(m member, ahead cell.ID, deck bool, heading geom.Vec) bool
	// true: by their boxes striking; false: by a step into a held cell refused
	byContact() bool

	other(id uid.UID64) (body, bool)
	onGoal(self, other body) bool
	stepAside(m member, of body) (MoveOrder, bool)
	detour(m member, o *MoveOrder, other body) bumpAnswer
	pass(m member, o *MoveOrder, other body)
	settle(m member, o *MoveOrder, other body) bool
}

// times as long as on the level a step aside may take at most (board.Map.Climb)
const yieldClimb = 3

type holders interface {
	Holder(c cell.ID, deck bool, d cell.Domain) (uid.UID64, bool)
}

type bumpAnswer uint8

const (
	stopAndPlan bumpAnswer = iota
	carryOn
	giveUp
)

type member struct {
	id     uid.UID64
	cell   cell.ID
	from   cell.ID // where a new route sets off: its cell, or where its step ends
	leg    Leg
	domain cell.Domain
	pos    world.Position
	vel    geom.Vec // world units a second
	facing geom.Vec // the way it faces, standing too
	z      world.Z
	lift   float64
	brake  float64 // world units a second²
	order  *MoveOrder
	// who came at m last tick, a step into m's cell refused
	pressedBy uid.UID64
	pressed   bool
	owners    tag.Tags[owner.Family]
}

func (m member) centre() geom.Vec { return m.pos.Center() }

// the cells a route goes round; blocked asks about them, the method value bound once
type avoiding struct {
	target  cell.ID
	known   [4]cell.ID
	n       uint8
	blocked func(cell.ID) bool
}

func (a *avoiding) avoids(c cell.ID) bool {
	return c != a.target && slices.Contains(a.known[:a.n], c)
}

// round the cells learned unless that found nothing within o's Retry, else any route; the first
// of the two run says whether o waits
func (a *avoiding) route(f *pathFinder, m member, from cell.ID, o *MoveOrder) (Path, bool) {
	due := o.Retry == 0
	if o.Avoids > 0 && due {
		a.target, a.known, a.n = o.Target, o.Avoid, o.Avoids
		if p, ok := f.findPathAround(m.id, m.domain, m.body(), from, o.Target, o.Level,
			a.blocked); ok {
			o.found()
			return p, true
		}
		o.failed()
	}
	p, ok := f.findPath(m.id, m.domain, m.body(), from, o.Target, o.Level)
	switch {
	case o.Avoids > 0:
	case ok:
		o.found()
	case due:
		o.failed()
	}
	return p, ok
}

type cellKeeping struct {
	finder *pathFinder
	occ    cell.Occupancy
	who    holders // nil: nobody is asked off a cell
	index  bodyIndex
	avoid  avoiding
	// whom held answers for, through the method value bound once
	holder   uid.UID64
	holderIn cell.Domain
	heldBy   func(c cell.ID, deck bool) bool
	asides   [8]scored
}

var _ keeping = (*cellKeeping)(nil)

func newCellKeeping(finder *pathFinder, occ cell.Occupancy) *cellKeeping {
	k := &cellKeeping{finder: finder, occ: occ, index: bodyIndex{grid: finder.grid}}
	k.who, _ = occ.(holders)
	k.avoid.blocked, k.heldBy = k.avoid.avoids, k.isHeld
	return k
}

func (k *cellKeeping) occupancy() cell.Occupancy { return k.occ }

func (k *cellKeeping) begin(gather func([]body) []body) {
	k.index.build(gather(k.index.bodies[:0]))
}

func (k *cellKeeping) orders(members []member, cmd MoveTo, issue func(member, MoveOrder)) {
	target := cmd.Cell
	var moves []member
	for _, m := range members {
		if cmd.Append && m.order != nil {
			m.order.Enqueue(Goal{Cell: target, At: cmd.At, Level: cmd.Level})
			continue
		}
		standing := m.order == nil || !m.order.Leg.Active
		if !cmd.Append && standing && m.cell == target && cmd.At != (geom.Vec{}) &&
			k.finder.on(m, target, cmd.Level) {
			issue(m, MoveOrder{Target: target, Face: cmd.At})
			continue
		}
		moves = append(moves, m)
	}
	pf := k.finder
	slices.SortStableFunc(moves, func(a, b member) int {
		return cmp.Compare(pf.grid.Distance(a.from, target), pf.grid.Distance(b.from, target))
	})
	taken := make(map[cell.ID]bool)
	for n, m := range moves {
		dest := target
		var path Path
		ok := false
		if n == 0 {
			path, ok = pf.findPath(m.id, m.domain, m.body(), m.from, target, cmd.Level)
		}
		if !ok {
			dest, path, ok = pf.nearestFree(m.id, m.domain, m.body(), m.from, target, cmd.Level,
				func(c cell.ID, _ bool) bool { return taken[c] })
		}
		if !ok {
			continue
		}
		taken[dest] = true
		issue(m, MoveOrder{Target: dest, Level: cmd.Level, Path: path, Leg: m.leg})
	}
}

func (k *cellKeeping) look(m member, at geom.Vec) MoveOrder {
	order := MoveOrder{Target: m.cell, Face: at}
	if m.leg.Active {
		order.Target, order.Leg = m.leg.To, m.leg
	}
	return order
}

func (k *cellKeeping) ready(member, *MoveOrder) {}

func (k *cellKeeping) route(m member, from cell.ID, o *MoveOrder) (Path, bool) {
	return k.avoid.route(k.finder, m, from, o)
}

func (k *cellKeeping) held(m member) func(c cell.ID, deck bool) bool {
	k.holder, k.holderIn = m.id, m.domain
	return k.heldBy
}

func (k *cellKeeping) isHeld(c cell.ID, deck bool) bool {
	return !k.occ.CanEnter(c, deck, k.holder, k.holderIn)
}

func (k *cellKeeping) lost(m member, from cell.ID, o *MoveOrder, waited time.Duration) (cell.ID, Path, bool, bool) {
	if waited < targetWaitTimeout {
		return 0, Path{}, true, false
	}
	dest, path, ok := k.finder.nearestFree(m.id, m.domain, m.body(), from, o.Target, o.Level,
		k.held(m))
	return dest, path, false, ok
}

func (k *cellKeeping) on(m member, c cell.ID, level Level) bool {
	return k.finder.on(m, c, level)
}

func (k *cellKeeping) steer(_ member, st steering.Helm, dir geom.Vec, speed float64) {
	if dir != (geom.Vec{}) {
		st.Request(dir)
	}
	st.RequestSpeed(speed)
}

func (k *cellKeeping) watch(m member, o *MoveOrder, _ cell.ID, _ geom.Vec, _ time.Duration) {
	o.Stalled = 0
	if m.cell == o.Toward {
		o.Stalls = 0
	}
}

// Held after stallAfter is the last word against rules that never move the unit
func (k *cellKeeping) blocked(m member, o *MoveOrder, c cell.ID, deck bool,
	d time.Duration) (uid.UID64, bool) {
	if o.Toward != c {
		o.Toward, o.Stalled = c, 0
	}
	var holder uid.UID64
	known := false
	if k.who != nil {
		holder, known = k.who.Holder(c, deck, m.domain)
	}
	if o.Stalled += d; o.Stalled >= stallAfter {
		o.Bumped, o.Held, o.Hit, o.HitUnit, o.Stalled = true, true, holder, known, 0
	}
	return holder, known
}

func (k *cellKeeping) bump(m member, o *MoveOrder) bumpAnswer {
	if !o.Held {
		return stopAndPlan
	}
	o.Held, o.Hit, o.HitUnit = false, 0, false
	if o.Stalls++; o.Stalls > maxStalls {
		return giveUp
	}
	if o.Toward == o.Target {
		dest, path, ok := k.finder.nearestFree(m.id, m.domain, m.body(), m.from, o.Target, o.Level,
			k.held(m))
		if !ok {
			return giveUp
		}
		o.Target, o.Path, o.Leg = dest, path, Leg{}
		o.found()
		return carryOn
	}
	o.learn(o.Toward)
	o.Path, o.Leg = Path{}, Leg{}
	return carryOn
}

func (k *cellKeeping) within(member) float64 { return arrivalEpsilon }

func (k *cellKeeping) mayStep(m member, ahead cell.ID, deck bool, _ geom.Vec) bool {
	return ahead == m.cell || k.occ.CanEnter(ahead, deck, m.id, m.domain)
}

func (k *cellKeeping) other(id uid.UID64) (body, bool) { return k.index.of(id) }

type scored struct {
	c     cell.ID
	score float64
}

// across the comer's way first, then behind, a slantwise step last; never ahead of it. Whether m
// may step there is the caller's to ask, in this order.
func (k *cellKeeping) aside(m member, from cell.ID, way geom.Vec) []scored {
	grid := k.finder.grid
	home := m.cell
	hc := grid.CellCenter(home)
	out := k.asides[:0]
	for i := range grid.Directions() {
		n, ok := grid.Toward(home, i)
		if !ok || n == from {
			continue
		}
		c := grid.CellCenter(n)
		d := geom.NewVec(c.X-hc.X, c.Y-hc.Y)
		l := math.Hypot(d.X, d.Y)
		if l == 0 {
			continue
		}
		dot := (d.X*way.X + d.Y*way.Y) / l
		if dot > 0.3 {
			continue
		}
		score := math.Abs(dot)
		if _, _, diagonal := grid.DiagonalNeighbors(home, n); diagonal {
			score++
		}
		out = append(out, scored{n, score})
	}
	slices.SortStableFunc(out, func(a, b scored) int { return cmp.Compare(a.score, b.score) })
	return out
}

func (k *cellKeeping) wayFrom(m member, asker body) (cell.ID, geom.Vec) {
	grid := k.finder.grid
	from, _ := grid.CellAt(asker.at)
	a, b := grid.CellCenter(from), grid.CellCenter(m.cell)
	d := geom.NewVec(b.X-a.X, b.Y-a.Y)
	if l := math.Hypot(d.X, d.Y); l > 0 {
		d = geom.NewVec(d.X/l, d.Y/l)
	}
	return from, d
}

func (k *cellKeeping) byContact() bool { return false }

func (k *cellKeeping) stepAside(m member, of body) (MoveOrder, bool) {
	from, way := k.wayFrom(m, of)
	terrain := k.finder.terrain
	for _, s := range k.aside(m, from, way) {
		c := s.c
		if !terrain.Kind(c).Admits(m.domain) || k.finder.climb(m.cell, c, m.domain) > yieldClimb {
			continue
		}
		if deck, ok := k.finder.stepsOnto(m, c); !ok || !k.occ.CanEnter(c, deck, m.id, m.domain) {
			continue
		}
		o := MoveOrder{Target: c, GivingWay: true}
		if m.facing != (geom.Vec{}) {
			c := m.centre()
			o.Face = geom.NewVec(c.X+m.facing.X*16, c.Y+m.facing.Y*16)
		}
		return o, true
	}
	return MoveOrder{}, false
}

// nothing to go round once m steps on, nor round other giving way: it is leaving the cell
func (k *cellKeeping) detour(m member, o *MoveOrder, other body) bumpAnswer {
	if o.Leg.Active || other.givingWay {
		return carryOn
	}
	c := o.Toward
	if c != o.Target && c != m.cell {
		o.learn(c)
	}
	if o.Cornered && o.Retry > 0 {
		o.Holding = 0
		return carryOn
	}
	path, ok := k.route(m, m.from, o)
	o.Holding, o.Cornered = 0, !ok || path.Index < path.Length && slices.Contains(path.Steps[path.Index:path.Length], c)
	if !o.Cornered {
		o.Path = path
	}
	return carryOn
}

func (k *cellKeeping) pass(member, *MoveOrder, body) {}

func (k *cellKeeping) onGoal(self, other body) bool { return other.cell == self.goal }

func (k *cellKeeping) settle(m member, o *MoveOrder, _ body) bool {
	dest, path, ok := k.finder.nearestFree(m.id, m.domain, m.body(), m.from, o.Target, o.Level,
		k.held(m))
	if !ok {
		return false
	}
	o.Target, o.Path, o.Leg = dest, path, Leg{}
	o.found()
	return true
}
