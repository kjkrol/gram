package navigation

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
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
	occupancy() cell.Occupancy
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
	route(m member, from cell.ID, o *MoveOrder) (Path, bool)
	// lost is what o does when no route reaches its Target, after waiting waited: waits on, settles
	// on dest along path, or, with neither, gives up.
	lost(m member, from cell.ID, o *MoveOrder, waited time.Duration) (dest cell.ID, path Path, wait, ok bool)
	// steer asks st for heading dir, zero keeping the one it has, and speed, clear of the others.
	steer(m member, st steering.Helm, dir geom.Vec, speed float64)
	// watch follows o's headway towards want, the point of toward, marking o Bumped when it has
	// made none for too long.
	watch(m member, o *MoveOrder, toward cell.ID, want geom.Vec, d time.Duration)
	// bump is m's answer to o marked Bumped: it struck the ground or someone bodily, or made no
	// headway — navigation's own last word, whatever the rules do.
	bump(m member, o *MoveOrder) bumpAnswer
	// blocked notes that m's step into c was refused, someone holding it, for d more: who holds it.
	blocked(m member, o *MoveOrder, c cell.ID, d time.Duration) (holder uid.UID64, known bool)
	// within is how near its goal m counts as there.
	within(m member) float64
	// mayStep reports whether m, driven by hand, may walk on into ahead along heading.
	mayStep(m member, ahead cell.ID, heading geom.Vec) bool
	// byContact reports whether units touch by their boxes striking — Collider's contacts — or,
	// false, by a step into a held cell refused.
	byContact() bool

	// What a Touch tells and what the commands a unit gives itself do:
	// other is whoever id was this tick.
	other(id uid.UID64) (body, bool)
	// onGoal reports whether other stands on self's goal.
	onGoal(self, other body) bool
	// stepAside is m's order to step off of's way, where the ground takes it, no steeper than
	// yieldClimb, and nobody stands.
	stepAside(m member, of body) (MoveOrder, bool)
	// detour has m, on the move, go round other.
	detour(m member, o *MoveOrder, other body) bumpAnswer
	// pass has m, on the move, go on past other, who makes way for it.
	pass(m member, o *MoveOrder, other body)
	// settle has o stand beside its goal, clear of other; false with nowhere to.
	settle(m member, o *MoveOrder, other body) bool
}

// yieldClimb is how many times as long as on the level a step aside may take at most, the slope's
// price (board.Map.Climb): a unit stepping aside never steps down a cliff nor up a wall.
const yieldClimb = 3

// holders is an Occupancy that tells who holds a cell: the board's.
type holders interface {
	Holder(c cell.ID, d cell.Domain) (uid.UID64, bool)
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
	cell   cell.ID // the cell it stands on
	from   cell.ID // where a new route sets off: its cell, or where its step ends
	leg    Leg     // the step in flight
	domain cell.Domain
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
	owners    tag.Tags[owner.Family]
}

// centre is the middle of m's box.
func (m member) centre() geom.Vec { return m.pos.Center() }

// cellKeeping gives a unit a cell to itself per domain, as the board's Occupancy says: a unit
// routes over the ground alone, not knowing where the others stand, and learns of them only when a
// step is refused — the cell held, a Touch of whoever holds it. What it does then is the rules'
// (Crowd); refused stallAfter, navigation routes round the cell itself, whatever they do.
type cellKeeping struct {
	finder *pathFinder
	occ    cell.Occupancy
	who    holders   // occ, when it tells who holds a cell; nil, nobody is asked off one
	index  bodyIndex // the tick's units: whether whoever holds a cell stands or passes
}

var _ keeping = (*cellKeeping)(nil)

// newCellKeeping holds cells in occ; finder plans the routes, blind to the others when its own
// occupancy is open.
func newCellKeeping(finder *pathFinder, occ cell.Occupancy) *cellKeeping {
	k := &cellKeeping{finder: finder, occ: occ, index: bodyIndex{grid: finder.grid}}
	k.who, _ = occ.(holders)
	return k
}

func (k *cellKeeping) occupancy() cell.Occupancy { return k.occ }

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
	taken := make(map[cell.ID]bool)
	for n, m := range moves {
		dest := target
		var path Path
		ok := false
		if n == 0 {
			path, ok = pf.findPath(m.id, m.domain, m.from, target)
		}
		if !ok {
			dest, path, ok = pf.nearestFree(m.id, m.domain, m.from, target, func(c cell.ID) bool { return taken[c] })
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
func (k *cellKeeping) route(m member, from cell.ID, o *MoveOrder) (Path, bool) {
	if o.Avoids > 0 {
		known := o.Avoid[:o.Avoids]
		blocked := func(c cell.ID) bool { return c != o.Target && slices.Contains(known, c) }
		if p, ok := k.finder.findPathAround(m.id, m.domain, from, o.Target, blocked); ok {
			return p, true
		}
	}
	return k.finder.findPath(m.id, m.domain, from, o.Target)
}

// held reports whether c is held against m.
func (k *cellKeeping) held(m member) func(cell.ID) bool {
	return func(c cell.ID) bool { return !k.occ.CanEnter(c, m.id, m.domain) }
}

// lost waits targetWaitTimeout for the target to free, then settles on the nearest free cell.
func (k *cellKeeping) lost(m member, from cell.ID, o *MoveOrder, waited time.Duration) (cell.ID, Path, bool, bool) {
	if waited < targetWaitTimeout {
		return 0, Path{}, true, false
	}
	dest, path, ok := k.finder.nearestFree(m.id, m.domain, from, o.Target, k.held(m))
	return dest, path, false, ok
}

func (k *cellKeeping) steer(_ member, st steering.Helm, dir geom.Vec, speed float64) {
	if dir != (geom.Vec{}) {
		st.Request(dir)
	}
	st.RequestSpeed(speed)
}

// watch sees a step under way, or the unit on its goal: no stall; reaching the cell it stalled
// for forgets the stalls.
func (k *cellKeeping) watch(m member, o *MoveOrder, _ cell.ID, _ geom.Vec, _ time.Duration) {
	o.Stalled = 0
	if m.cell == o.Toward {
		o.Stalls = 0
	}
}

// blocked notes m's step into c refused — someone holds it — and marks o Bumped and Held for
// bump to answer once it has been refused stallAfter: the last word against rules that never move
// it.
func (k *cellKeeping) blocked(m member, o *MoveOrder, c cell.ID, d time.Duration) (uid.UID64, bool) {
	if o.Toward != c {
		o.Toward, o.Stalled = c, 0
	}
	var holder uid.UID64
	known := false
	if k.who != nil {
		holder, known = k.who.Holder(c, m.domain)
	}
	if o.Stalled += d; o.Stalled >= stallAfter {
		o.Bumped, o.Held, o.Hit, o.HitUnit, o.Stalled = true, true, holder, known, 0
	}
	return holder, known
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

// mayStep lets m on within its own cell, and into another the occupancy lets it into.
func (k *cellKeeping) mayStep(m member, ahead cell.ID, _ geom.Vec) bool {
	return ahead == m.cell || k.occ.CanEnter(ahead, m.id, m.domain)
}

// other is whoever id was this tick.
func (k *cellKeeping) other(id uid.UID64) (body, bool) { return k.index.of(id) }

// aside is where m could step off the way of one coming from from along way: m's free neighbours
// and those held, its domain's ground, not steeper than yieldClimb, never ahead of the one coming
// nor into its cell — across its way first, then behind, a slantwise step last.
func (k *cellKeeping) aside(m member, from cell.ID, way geom.Vec) []cell.ID {
	grid, terrain := k.finder.grid, k.finder.terrain
	home := m.cell
	hc := grid.CellCenter(home)
	type scored struct {
		c     cell.ID
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
	cells := make([]cell.ID, len(out))
	for i, s := range out {
		cells[i] = s.c
	}
	return cells
}

// wayFrom is the way from asker's cell to m's, a unit vector.
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

// stepAside has m step to a free neighbour off the way of, its domain's ground no steeper than
// yieldClimb, facing as it did; with none it has no order.
func (k *cellKeeping) stepAside(m member, of body) (MoveOrder, bool) {
	from, way := k.wayFrom(m, of)
	for _, c := range k.aside(m, from, way) {
		if !k.occ.CanEnter(c, m.id, m.domain) {
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

// detour notes the cell m is refused — unless it is m's goal — for the routes to go round, and
// plans afresh: Cornered with no way round but through it, the route kept then. Once m steps on
// there is nothing to go round, nor round other giving way: it is leaving the cell.
func (k *cellKeeping) detour(m member, o *MoveOrder, other body) bumpAnswer {
	if o.Leg.Active || other.givingWay {
		return carryOn
	}
	c := o.Toward
	if c != o.Target && c != m.cell {
		o.learn(c)
	}
	path, ok := k.route(m, m.from, o)
	o.Holding, o.Cornered = 0, !ok || path.Index < path.Length && slices.Contains(path.Steps[path.Index:path.Length], c)
	if !o.Cornered {
		o.Path = path
	}
	return carryOn
}

// pass leaves m's step to wait for the cell other leaves.
func (k *cellKeeping) pass(member, *MoveOrder, body) {}

func (k *cellKeeping) onGoal(self, other body) bool { return other.cell == self.goal }

func (k *cellKeeping) settle(m member, o *MoveOrder, _ body) bool {
	dest, path, ok := k.finder.nearestFree(m.id, m.domain, m.from, o.Target, k.held(m))
	if !ok {
		return false
	}
	o.Target, o.Path, o.Leg = dest, path, Leg{}
	return true
}
