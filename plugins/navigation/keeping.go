package navigation

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// keeping is how navigation keeps units out of each other's way, as Spacing says: cellKeeping
// holds cells, bodyKeeping keeps boxes apart. The systems ask it where the two differ.
type keeping interface {
	// occupancy is what steps and routes hold and ask.
	occupancy() board.Occupancy
	// begin readies a tick; gather lists every unit as it stands, for a keeping that needs them.
	begin(gather func([]body) []body)
	// orders gives the members their orders towards cmd: issue hands a fresh one, and an appended
	// goal is queued on the member's own order.
	orders(members []member, cmd MoveTo, issue func(member, MoveOrder))
	// look is m's order to stop and turn towards at.
	look(m member, at geom.Vec) MoveOrder
	// ready readies o before m is steered by it; it may move o's Target and Spot.
	ready(m member, o *MoveOrder)
	// route is the way for m from from to o's Target.
	route(m member, from board.CellID, o *MoveOrder) (Path, bool)
	// lost is what o does when no route reaches its Target, after waiting waited: waits on, settles
	// on dest along path, or, with neither, gives up.
	lost(m member, from board.CellID, o *MoveOrder, waited time.Duration) (dest board.CellID, path Path, wait, ok bool)
	// steer asks st for heading dir, zero keeping the one it has, and speed, clear of the others.
	steer(m member, st *steering.Steering, dir geom.Vec, speed float64)
	// watch follows o's headway towards want, the point of toward, marking o Bumped when it has
	// made none for too long.
	watch(m member, o *MoveOrder, toward board.CellID, want geom.Vec, d time.Duration)
	// bump is m's answer to o marked Bumped: it struck someone, or made no headway.
	bump(m member, o *MoveOrder) bumpAnswer
	// blocked notes that m's step into c was refused, someone holding it, for d more: who holds it,
	// and whether to ask them off it.
	blocked(m member, o *MoveOrder, c board.CellID, d time.Duration) (holder uid.UID64, known, asks bool)
	// within is how near its goal m counts as there.
	within(m member) float64
	// mayStep reports whether m, driven by hand, may walk on into ahead along heading.
	mayStep(m member, ahead board.CellID, heading geom.Vec) bool
	// yield is the order m, standing, takes to give way to the first of presses coming at it on
	// the move; false for none.
	yield(m member, presses []press) (MoveOrder, bool)

	// For the units with a tree (plugins/world/act):
	// other is whoever id was this tick.
	other(id uid.UID64) (body, bool)
	// room is whether m, standing, has somewhere free to step to off asker's way — no farther
	// than beside, no steeper than yieldClimb — or else the ally standing where it could.
	room(m member, asker body) (free bool, ally uid.UID64, beside bool)
	// stepAside is m's order to step there, and home again after a while when back.
	stepAside(m member, asker body, back bool) (MoveOrder, bool)
	// onGoal reports whether other stands on o's goal.
	onGoal(m member, o *MoveOrder, other body) bool
	// settle has o stand beside its goal, clear of other; false with nowhere to.
	settle(m member, o *MoveOrder, other body) bool
}

// yieldClimb is how many times as long as on the level a step aside may take at most, the slope's
// price (board.Map.Climb): a unit asked to make way never steps down a cliff nor up a wall.
const yieldClimb = 3

// treeGrace is how long a unit with a tree stands at a step refused before navigation
// goes round for it, whatever its tree does: the last word against a tree that never moves it.
const treeGrace = 5 * stallAfter

// press is someone on the move coming at a standing unit: who, from which cell, which way — the
// way they come, a unit vector — and whether they are giving way themselves, whom nobody gives
// way to in turn. Under BodySpacing a press is a contact struck, under CellSpacing a step into
// the standing unit's cell refused.
type press struct {
	other     uid.UID64
	cell      board.CellID
	way       geom.Vec
	givingWay bool
}

// holders is an Occupancy that tells who holds a cell: the board's.
type holders interface {
	Holder(c board.CellID, d board.Domain) (uid.UID64, bool)
}

// bumpAnswer is what a unit does about a bump.
type bumpAnswer uint8

const (
	// stopAndPlan stops the unit where it stands and plans its route again from there.
	stopAndPlan bumpAnswer = iota
	// carryOn has it go on, as the keeping has set its order.
	carryOn
	// giveUp ends its order where it stands.
	giveUp
)

// member is a unit as a command or a tick sees it.
type member struct {
	id     uid.UID64
	cell   board.CellID // the cell it stands on
	from   board.CellID // where a new route sets off: its cell, or where its step ends
	leg    Leg          // the step in flight
	domain board.Domain
	pos    world.Position
	vel    geom.Vec // world units a second
	facing geom.Vec // the way it faces, standing too
	z      world.Z
	lift   float64
	brake  float64    // world units a second² it slows at
	order  *MoveOrder // nil when it has none
	// pressedBy is who came at m last tick — a step into m's cell refused — when pressed.
	pressedBy uid.UID64
	pressed   bool
	// minded is a unit with a tree (act.Mind): its tree, not navigation, decides whom to
	// ask, when to go round and where to stand instead.
	minded bool
	owners tag.Tags[owner.Family]
}

// centre is the middle of m's box.
func (m member) centre() geom.Vec { return board.Center(m.pos) }

// cellKeeping gives a unit a cell to itself per domain, as the board's Occupancy says, and keeps
// units out of each other's way as bodyKeeping does: a unit routes over the ground alone, not
// knowing where the others stand, and learns of them only when a step is refused — the cell
// held; it waits, asks the one standing there off it, and after stallAfter of no headway routes
// round the cell it learnt of. Someone standing on its goal it waits targetWaitTimeout for, then
// settles beside.
type cellKeeping struct {
	finder *pathFinder
	occ    board.Occupancy
	who    holders   // occ, when it tells who holds a cell; nil, nobody is asked off one
	index  bodyIndex // the tick's units: whether whoever holds a cell stands or passes
}

var _ keeping = (*cellKeeping)(nil)

// newCellKeeping holds cells in occ; finder plans the routes, blind to the others when its own
// occupancy is open.
func newCellKeeping(finder *pathFinder, occ board.Occupancy) *cellKeeping {
	k := &cellKeeping{finder: finder, occ: occ, index: bodyIndex{grid: finder.grid}}
	k.who, _ = occ.(holders)
	return k
}

func (k *cellKeeping) occupancy() board.Occupancy { return k.occ }

func (k *cellKeeping) begin(gather func([]body) []body) {
	k.index.build(gather(k.index.bodies[:0]))
}

// orders gives each member its own free cell at or around cmd.Cell, nearest first, or with Append
// queues the cell; one standing on the target turns towards the point clicked instead.
func (k *cellKeeping) orders(members []member, cmd MoveTo, issue func(member, MoveOrder)) {
	target := cmd.Cell
	var moves []member
	for _, m := range members {
		if cmd.Append && m.order != nil {
			m.order.Enqueue(Goal{Cell: target, At: cmd.At})
			continue
		}
		standing := m.order == nil || !m.order.Leg.Active
		if !cmd.Append && standing && m.cell == target && cmd.At != (geom.Vec{}) {
			issue(m, MoveOrder{Target: target, Face: cmd.At})
			continue
		}
		moves = append(moves, m)
	}
	pf := k.finder
	slices.SortStableFunc(moves, func(a, b member) int {
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
			dest, path, ok = pf.nearestFree(m.id, m.domain, m.from, target, func(c board.CellID) bool { return taken[c] })
		}
		if !ok {
			continue
		}
		taken[dest] = true
		issue(m, MoveOrder{Target: dest, Path: path, Leg: m.leg})
	}
}

// look has m finish the step it is on and turn.
func (k *cellKeeping) look(m member, at geom.Vec) MoveOrder {
	order := MoveOrder{Target: m.cell, Face: at}
	if m.leg.Active {
		order.Target, order.Leg = m.leg.To, m.leg
	}
	return order
}

func (k *cellKeeping) ready(member, *MoveOrder) {}

// route is the way to o's Target round the cells m found someone standing in.
func (k *cellKeeping) route(m member, from board.CellID, o *MoveOrder) (Path, bool) {
	if o.Avoids > 0 {
		known := o.Avoid[:o.Avoids]
		blocked := func(c board.CellID) bool { return c != o.Target && slices.Contains(known, c) }
		if p, ok := k.finder.findPathAround(m.id, m.domain, from, o.Target, blocked); ok {
			return p, true
		}
	}
	return k.finder.findPath(m.id, m.domain, from, o.Target)
}

// held reports whether c is held against m.
func (k *cellKeeping) held(m member) func(board.CellID) bool {
	return func(c board.CellID) bool { return !k.occ.CanEnter(c, m.id, m.domain) }
}

// lost waits targetWaitTimeout for the target to free, then settles on the nearest free cell.
func (k *cellKeeping) lost(m member, from board.CellID, o *MoveOrder, waited time.Duration) (board.CellID, Path, bool, bool) {
	if waited < targetWaitTimeout {
		return 0, Path{}, true, false
	}
	dest, path, ok := k.finder.nearestFree(m.id, m.domain, from, o.Target, k.held(m))
	return dest, path, false, ok
}

func (k *cellKeeping) steer(_ member, st *steering.Steering, dir geom.Vec, speed float64) {
	if dir != (geom.Vec{}) {
		st.Request(dir)
	}
	st.RequestSpeed(speed)
}

// watch sees a step under way, or the unit on its goal: no stall; reaching the cell it stalled
// for forgets the stalls.
func (k *cellKeeping) watch(m member, o *MoveOrder, _ board.CellID, _ geom.Vec, _ time.Duration) {
	o.Stalled = 0
	if m.cell == o.Toward {
		o.Stalls = 0
	}
}

// blocked has m wait for the cell, asking whoever stands there off it — not off m's own goal,
// where it settles beside instead — and marks o Bumped and Held for bump to answer once it has
// waited stallAfter; targetWaitTimeout for its goal with someone standing on it; not at all
// where the two came at each other's cells and m's id is the lower, since the other goes round
// at once.
func (k *cellKeeping) blocked(m member, o *MoveOrder, c board.CellID, d time.Duration) (uid.UID64, bool, bool) {
	if o.Toward != c {
		o.Toward, o.Stalled = c, 0
	}
	var holder uid.UID64
	known := false
	if k.who != nil {
		holder, known = k.who.Holder(c, m.domain)
	}
	standing := true
	if b, ok := k.index.of(holder); known && ok {
		standing = !b.moving
	}
	wait := stallAfter
	switch {
	case m.minded:
		wait = treeGrace // its tree decides; navigation goes round only if it never does
	case known && m.pressed && m.pressedBy == holder && m.id > holder:
		wait = 0 // head on: the one with the greater id goes round, the other waits
	case known && m.pressed && m.pressedBy == holder:
		wait = time.Duration(math.MaxInt64)
	case c == o.Target && standing:
		wait = targetWaitTimeout
	}
	if o.Stalled += d; o.Stalled >= wait {
		o.Bumped, o.Held, o.Hit, o.HitUnit, o.Stalled = true, true, holder, known, 0
	}
	if c == o.Target || m.minded {
		return holder, known, false
	}
	return holder, known, known
}

// bump answers a step refused for too long — o Held — by learning the cell for the routes to go
// round and planning afresh, or, when it is m's goal someone stands on, by settling on the nearest
// free cell; after maxStalls it gives up. Struck bodily, m stops and plans again as ever.
func (k *cellKeeping) bump(m member, o *MoveOrder) bumpAnswer {
	if !o.Held {
		return stopAndPlan
	}
	o.Held, o.Hit, o.HitUnit = false, 0, false
	if o.Stalls++; o.Stalls > maxStalls {
		return giveUp
	}
	if o.Toward == o.Target {
		dest, path, ok := k.finder.nearestFree(m.id, m.domain, m.from, o.Target, k.held(m))
		if !ok {
			return giveUp
		}
		o.Target, o.Path, o.Leg = dest, path, Leg{}
		return carryOn
	}
	o.learn(o.Toward)
	o.Path, o.Leg = Path{}, Leg{}
	return carryOn
}

func (k *cellKeeping) within(member) float64 { return arrivalEpsilon }

// yield has m, standing, step to a free neighbouring cell off the way of the first one coming at
// it — not one giving way itself: across its way first, then behind, never ahead of it nor into
// its cell — stand there yieldLinger and go back to its own cell, facing as it did. With no such
// cell it holds.
func (k *cellKeeping) yield(m member, presses []press) (MoveOrder, bool) {
	grid, terrain := k.finder.grid, k.finder.terrain
	for _, p := range presses {
		if p.givingWay {
			continue
		}
		home := m.cell
		hc := grid.CellCenter(home)
		var best board.CellID
		found, along := false, math.Inf(1)
		for _, n := range grid.Neighbors(home) {
			if n == p.cell || !terrain.Kind(n).Admits(m.domain) || !k.occ.CanEnter(n, m.id, m.domain) {
				continue
			}
			c := grid.CellCenter(n)
			d := geom.NewVec(c.X-hc.X, c.Y-hc.Y)
			l := math.Hypot(d.X, d.Y)
			if l == 0 {
				continue
			}
			dot := (d.X*p.way.X + d.Y*p.way.Y) / l
			if dot > 0.3 {
				continue // on the way of the one coming
			}
			score := math.Abs(dot) // across first, then behind, a slantwise step last: it holds the corners too
			if _, _, diagonal := grid.DiagonalNeighbors(home, n); diagonal {
				score++
			}
			if score < along {
				best, along, found = n, score, true
			}
		}
		if !found {
			return MoveOrder{}, false
		}
		o := MoveOrder{Target: best, Linger: yieldLinger, GivingWay: true}
		o.Enqueue(Goal{Cell: home})
		if m.facing != (geom.Vec{}) {
			c := m.centre()
			o.Face = geom.NewVec(c.X+m.facing.X*16, c.Y+m.facing.Y*16)
		}
		return o, true
	}
	return MoveOrder{}, false
}

// mayStep lets m on within its own cell, and into another the occupancy lets it into.
func (k *cellKeeping) mayStep(m member, ahead board.CellID, _ geom.Vec) bool {
	return ahead == m.cell || k.occ.CanEnter(ahead, m.id, m.domain)
}

// other is whoever id was this tick.
func (k *cellKeeping) other(id uid.UID64) (body, bool) { return k.index.of(id) }

// aside is where m could step off the way of one coming from from along way: m's free neighbours
// and those held, its domain's ground, not steeper than yieldClimb, never ahead of the one coming
// nor into its cell — across its way first, then behind, a slantwise step last.
func (k *cellKeeping) aside(m member, from board.CellID, way geom.Vec) []board.CellID {
	grid, terrain := k.finder.grid, k.finder.terrain
	home := m.cell
	hc := grid.CellCenter(home)
	type scored struct {
		c     board.CellID
		score float64
	}
	var out []scored
	for _, n := range grid.Neighbors(home) {
		if n == from || !terrain.Kind(n).Admits(m.domain) || k.finder.climb(home, n, m.domain) > yieldClimb {
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
	cells := make([]board.CellID, len(out))
	for i, s := range out {
		cells[i] = s.c
	}
	return cells
}

// wayFrom is the way from asker's cell to m's, a unit vector.
func (k *cellKeeping) wayFrom(m member, asker body) (board.CellID, geom.Vec) {
	grid := k.finder.grid
	from, _ := grid.CellAt(asker.at)
	a, b := grid.CellCenter(from), grid.CellCenter(m.cell)
	d := geom.NewVec(b.X-a.X, b.Y-a.Y)
	if l := math.Hypot(d.X, d.Y); l > 0 {
		d = geom.NewVec(d.X/l, d.Y/l)
	}
	return from, d
}

func (k *cellKeeping) room(m member, asker body) (bool, uid.UID64, bool) {
	from, way := k.wayFrom(m, asker)
	var ally uid.UID64
	beside := false
	for _, c := range k.aside(m, from, way) {
		if k.occ.CanEnter(c, m.id, m.domain) {
			return true, 0, false
		}
		if beside || k.who == nil {
			continue
		}
		if h, ok := k.who.Holder(c, m.domain); ok {
			if b, ok := k.index.of(h); ok && !b.moving && owner.Allies(m.owners, b.owners) {
				ally, beside = h, true
			}
		}
	}
	return false, ally, beside
}

func (k *cellKeeping) stepAside(m member, asker body, back bool) (MoveOrder, bool) {
	from, way := k.wayFrom(m, asker)
	for _, c := range k.aside(m, from, way) {
		if !k.occ.CanEnter(c, m.id, m.domain) {
			continue
		}
		o := MoveOrder{Target: c, GivingWay: true}
		if back {
			o.Linger = yieldLinger
			o.Enqueue(Goal{Cell: m.cell})
			if m.facing != (geom.Vec{}) {
				c := m.centre()
				o.Face = geom.NewVec(c.X+m.facing.X*16, c.Y+m.facing.Y*16)
			}
		}
		return o, true
	}
	return MoveOrder{}, false
}

func (k *cellKeeping) onGoal(_ member, o *MoveOrder, other body) bool {
	c, ok := k.finder.grid.CellAt(other.at)
	return ok && c == o.Target
}

func (k *cellKeeping) settle(m member, o *MoveOrder, _ body) bool {
	dest, path, ok := k.finder.nearestFree(m.id, m.domain, m.from, o.Target, k.held(m))
	if !ok {
		return false
	}
	o.Target, o.Path, o.Leg = dest, path, Leg{}
	return true
}
