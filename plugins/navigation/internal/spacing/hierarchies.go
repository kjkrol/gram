package spacing

import (
	"log"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
)

// the most classes of units whose hierarchies are kept at once, the least lately asked for let go
// first
const keptHierarchies = 4

// the work an Update spends building a hierarchy, in hpastar's steps: a step of a cluster of level
// 1 costs about 33 ns, its board's steps read over relief, and about 11 ns above
// (PathFinder_Hierarchy's board), so 2.5 ms / 33 ns ≈ 75,000 keeps a tick building within 2-3 ms
const buildBudget = 75_000

// a board counting the changes to what a step over its cells reads, all of them and a cell's
type passages interface {
	Passage() uint64
	CellPassage(c cell.ID) uint64
}

type hierarchyEntry struct {
	key   pathfind.Key
	h     *pathfind.Hierarchy // nil: a free place
	since uint64              // the board's passage its clusters were last marked stale against
	used  uint64              // the Update it was last asked for
	ready bool                // none of its clusters stale: routes are climbed through it
}

// Hierarchies are the hierarchies of the units' classes whose far routes are climbed, at most
// keptHierarchies, the least lately asked for let go first. An Update has them asked for, staled
// and built on its own goroutine before any ask is answered; routers answering on several
// goroutines read them as they stand.
type Hierarchies struct {
	finder  *Finder  // the one they are built through; nil: none kept
	board   passages // the passage of the finder's board
	side    int
	from    float64
	budget  int // the work a Sweep builds, buildBudget
	update  uint64
	entries [keptHierarchies]hierarchyEntry
	dropped bool // a hierarchy let go for another's was said in the log
}

// Bind has hs keep hierarchies of clusters side cells a side, built through f, for routes whose
// ends lie at least from cells apart: f blind to the others and planning by levels over a grid of
// grid.DefaultGrids and a board counting the passage of its cells. Over any other, or a side of
// 0, none.
func (hs *Hierarchies) Bind(f *Finder, side int, from float64) {
	*hs = Hierarchies{side: side, from: from, budget: buildBudget, update: 1}
	if _, blind := f.Occupancy.(OpenOccupancy); !blind || side <= 0 || !f.leveled ||
		!laidOut(f.grid) {
		return
	}
	if b, ok := f.Terrain.(passages); ok {
		hs.finder, hs.board = f, b
	}
}

// whether g is a grid of grid.DefaultGrids, which a hierarchy is laid over; a board's read says
// it of the grid it embeds
func laidOut(g grid.Grid) bool {
	if s, ok := g.(interface{ Shape() (grid.Shape, bool) }); ok {
		_, ok = s.Shape()
		return ok
	}
	_, ok := grid.ShapeOf(g)
	return ok
}

// On reports whether hs keeps hierarchies.
func (hs *Hierarchies) On() bool { return hs.finder != nil }

// Far reports whether the route from from to to is climbed, its ends lying at least as far apart
// as hs was bound with: the question alone says it, so any router answers it alike.
func (hs *Hierarchies) Far(from, to cell.ID) bool {
	return hs.finder != nil && hs.finder.grid.Distance(from, to) >= hs.from
}

// Key is the class of units m's hierarchy is built for: its domain and its body, its feet left
// out; false where hs keeps none.
func (hs *Hierarchies) Key(m bodies.Member) (pathfind.Key, bool) {
	if hs.finder == nil {
		return pathfind.Key{}, false
	}
	body := m.Body()
	body.Feet = 0
	return pathfind.Key{Domain: m.Domain, Body: body}, true
}

// Expect has key's hierarchy wanted this Update: one kept is kept on; a new one takes a free
// place, else that of the least lately asked for, none where all are wanted.
func (hs *Hierarchies) Expect(key pathfind.Key) {
	if hs.finder == nil {
		return
	}
	var spare *hierarchyEntry
	for i := range hs.entries {
		e := &hs.entries[i]
		if e.h != nil && e.key == key {
			e.used = hs.update
			return
		}
		if hs.sparer(e, spare) {
			spare = e
		}
	}
	if spare == nil {
		return
	}
	if spare.h != nil && !hs.dropped {
		log.Printf("navigation: more classes of units ask for far routes than the %d whose "+
			"hierarchies are kept: the least lately asked for is let go", keptHierarchies)
		hs.dropped = true
	}
	*spare = hierarchyEntry{key: key, h: hs.finder.NewHierarchy(key, hs.side),
		since: hs.board.Passage(), used: hs.update}
}

// whether e makes a better place for a new hierarchy than spare: a free one, the first; with
// none free, the one not wanted this Update and least lately before
func (hs *Hierarchies) sparer(e, spare *hierarchyEntry) bool {
	switch {
	case e.h == nil:
		return spare == nil || spare.h != nil
	case e.used < hs.update:
		return spare == nil || spare.h != nil && e.used < spare.used
	}
	return false
}

// Due reports whether a Sweep has clusters to mark stale or to build.
func (hs *Hierarchies) Due() bool {
	if hs.finder == nil {
		return false
	}
	v := hs.board.Passage()
	for i := range hs.entries {
		if e := &hs.entries[i]; e.h != nil && (!e.ready || e.since != v) {
			return true
		}
	}
	return false
}

// Sweep readies the hierarchies for an Update's answers: each is marked stale where the board's
// passage moved, as a climb asks, and the one not ready asked for most lately is built
// buildBudget steps of hpastar's work further, the same work on any machine.
func (hs *Hierarchies) Sweep() {
	if hs.finder == nil {
		return
	}
	v := hs.board.Passage()
	for i := range hs.entries {
		if e := &hs.entries[i]; e.h != nil && e.since != v {
			hs.finder.Stale(e.h, e.since)
			e.since, e.ready = v, e.h.Ready()
		}
	}
	if k := hs.unbuilt(); k >= 0 {
		e := &hs.entries[k]
		hs.finder.Build(e.h, hs.budget)
		e.ready = e.h.Ready()
	}
	hs.update++
}

// unbuilt is the entry of the hierarchy not ready asked for most lately, the first of those
// asked for alike; -1 for none
func (hs *Hierarchies) unbuilt() int {
	k := -1
	for i := range hs.entries {
		e := &hs.entries[i]
		if e.h != nil && !e.ready && (k < 0 || e.used > hs.entries[k].used) {
			k = i
		}
	}
	return k
}

// Take is key's hierarchy where it is built, for an ask's route to be climbed through it; nil
// while it is not, the route searched. Routers on several goroutines take it at once.
func (hs *Hierarchies) Take(key pathfind.Key) *pathfind.Hierarchy {
	for i := range hs.entries {
		if e := &hs.entries[i]; e.ready && e.key == key {
			return e.h
		}
	}
	return nil
}
