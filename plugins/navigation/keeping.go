package navigation

import (
	"cmp"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
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
	// within is how near its goal m counts as there.
	within(m member) float64
	// mayStep reports whether m, driven by hand, may walk on into ahead along heading.
	mayStep(m member, ahead board.CellID, heading geom.Vec) bool
	// yield is the order m, standing, takes to give way to one on the move it was struck by, as
	// contacts say; false for none.
	yield(m member, contacts []collision.Contact) (MoveOrder, bool)
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
}

// centre is the middle of m's box.
func (m member) centre() geom.Vec { return board.Center(m.pos) }

// cellKeeping gives a unit a cell to itself per domain, as the board's Occupancy says.
type cellKeeping struct {
	finder *pathFinder
	occ    board.Occupancy
}

var _ keeping = (*cellKeeping)(nil)

func newCellKeeping(finder *pathFinder) *cellKeeping {
	return &cellKeeping{finder: finder, occ: finder.occupancy}
}

func (k *cellKeeping) occupancy() board.Occupancy { return k.occ }

func (k *cellKeeping) begin(func([]body) []body) {}

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
			dest, path, ok = pf.nearestFree(m.id, m.domain, m.from, target, taken)
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

func (k *cellKeeping) route(m member, from board.CellID, o *MoveOrder) (Path, bool) {
	return k.finder.findPath(m.id, m.domain, from, o.Target)
}

// lost waits targetWaitTimeout for the target to free, then settles on the nearest free cell.
func (k *cellKeeping) lost(m member, from board.CellID, o *MoveOrder, waited time.Duration) (board.CellID, Path, bool, bool) {
	if waited < targetWaitTimeout {
		return 0, Path{}, true, false
	}
	dest, path, ok := k.finder.nearestFree(m.id, m.domain, from, o.Target, nil)
	return dest, path, false, ok
}

func (k *cellKeeping) steer(_ member, st *steering.Steering, dir geom.Vec, speed float64) {
	if dir != (geom.Vec{}) {
		st.Request(dir)
	}
	st.RequestSpeed(speed)
}

func (k *cellKeeping) watch(member, *MoveOrder, board.CellID, geom.Vec, time.Duration) {}

// bump stops the unit and plans again round the cells now held.
func (k *cellKeeping) bump(member, *MoveOrder) bumpAnswer { return stopAndPlan }

func (k *cellKeeping) within(member) float64 { return arrivalEpsilon }

// yield gives no way: a unit holds its cell.
func (k *cellKeeping) yield(member, []collision.Contact) (MoveOrder, bool) { return MoveOrder{}, false }

// mayStep lets m on within its own cell, and into another the occupancy lets it into.
func (k *cellKeeping) mayStep(m member, ahead board.CellID, _ geom.Vec) bool {
	return ahead == m.cell || k.occ.CanEnter(ahead, m.id, m.domain)
}
