package spacing

import (
	"sync"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
)

// the most goals whose fields are kept at once, the least lately asked for let go first
const keptFields = 8

// how much farther than the farthest unit it is filled for a field reaches, for one pushed back
// on its way
const fieldMargin = 1.5

// what a field is filled for: the goal of a group, the level it ends on, the domain and the
// body's class, its feet left out
type fieldAsked struct {
	goal   cell.ID
	end    order.Level
	domain cell.Domain
	body   pathfind.Body
}

type fieldState uint8

const (
	fieldFree     fieldState = iota
	fieldExpected            // asked for this Update, its seeds gathered, not yet filled
	fieldBuilding
	fieldDone
)

type fieldEntry struct {
	key   fieldAsked
	field pathfind.Field
	seeds []cell.ID
	state fieldState
	used  uint64 // the Update it was last asked for
}

// Fields are the fields of the groups' goals whose routes are read off them, at most keptFields,
// all let go when the board changes. An Update has them expected before any ask is answered;
// routers answering on several goroutines share them, one filling what the others wait for.
type Fields struct {
	mu      sync.Mutex
	worked  sync.Cond // a field filled
	grid    grid.Grid
	board   interface{ Version() uint64 } // nil: no fields kept
	version uint64
	update  uint64
	entries [keptFields]fieldEntry
}

// Bind has fs keep fields filled through finders as f searches: blind to the others, over a
// board counting its changes; over any other, none.
func (fs *Fields) Bind(f *Finder) {
	fs.grid, fs.board, fs.worked.L = f.grid, nil, &fs.mu
	if _, blind := f.Occupancy.(OpenOccupancy); blind {
		fs.board, _ = f.Terrain.(interface{ Version() uint64 })
	}
}

// Key is what the field of the goal of m's group is filled for, o asking; false where o is no
// group's or giving way, with its goals queued behind, or its point lies off the board.
func (fs *Fields) Key(m bodies.Member, o *order.MoveOrder) (fieldAsked, bool) {
	if fs.board == nil || o.Group == 0 || o.At == (geom.Vec{}) || o.GivingWay && o.Queued > 0 {
		return fieldAsked{}, false
	}
	goal, ok := fs.grid.CellAt(o.At)
	if !ok {
		return fieldAsked{}, false
	}
	body := m.Body()
	body.Feet = 0
	return fieldAsked{goal: goal, end: o.Level, domain: m.Domain, body: body}, true
}

// Sweep begins an Update's asks: a board changed since the last lets every field go.
func (fs *Fields) Sweep() {
	if fs.board == nil {
		return
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	v := fs.board.Version()
	for i := range fs.entries {
		if e := &fs.entries[i]; v != fs.version || e.state == fieldExpected {
			e.state = fieldFree
		}
	}
	fs.version = v
	fs.update++
}

// Expect has q's field wanted this Update, reaching seed: one filled already is kept as it is;
// a new one takes a free place, else that of the least lately asked for, none where all are
// wanted.
func (fs *Fields) Expect(q fieldAsked, seed cell.ID) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	var spare *fieldEntry
	for i := range fs.entries {
		e := &fs.entries[i]
		if e.state != fieldFree && e.key == q {
			if e.state == fieldExpected {
				e.seeds = append(e.seeds, seed)
			}
			e.used = fs.update
			return
		}
		if fs.sparer(e, spare) {
			spare = e
		}
	}
	if spare == nil {
		return
	}
	spare.key, spare.state, spare.used = q, fieldExpected, fs.update
	spare.seeds = append(spare.seeds[:0], seed)
}

// whether e makes a better place for a new field than spare: the free one let go latest, whose
// arrays are grown, so a field filled again after the board changed allocates nothing; with none
// free, the filled one not asked for this Update and least lately before
func (fs *Fields) sparer(e, spare *fieldEntry) bool {
	switch {
	case e.state == fieldFree:
		return spare == nil || spare.state != fieldFree || e.used > spare.used
	case e.state == fieldDone && e.used < fs.update:
		return spare == nil || spare.state == fieldDone && e.used < spare.used
	}
	return false
}

// Take is q's field this Update: filled, or once another fills it, or with build to fill (Fill)
// and give (Give) itself; nil where q has none.
func (fs *Fields) Take(q fieldAsked) (f *pathfind.Field, build bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for i := range fs.entries {
		e := &fs.entries[i]
		if e.state == fieldFree || e.key != q {
			continue
		}
		for e.state == fieldBuilding {
			fs.worked.Wait()
		}
		if e.state == fieldExpected {
			e.state = fieldBuilding
			return &e.field, true
		}
		return &e.field, false
	}
	return nil, false
}

// Fill fills f, taken to build, through finder for what it is asked for, from its seeds.
func (fs *Fields) Fill(f *pathfind.Field, finder *Finder) {
	e := fs.entry(f)
	finder.FillField(f, e.key.goal, pathfind.Level(e.key.end), e.key.domain, e.key.body, e.seeds,
		fieldMargin)
}

// Give has f, filled, read by every ask for it, those waiting too.
func (fs *Fields) Give(f *pathfind.Field) {
	fs.mu.Lock()
	fs.entry(f).state = fieldDone
	fs.mu.Unlock()
	fs.worked.Broadcast()
}

func (fs *Fields) entry(f *pathfind.Field) *fieldEntry {
	for i := range fs.entries {
		if &fs.entries[i].field == f {
			return &fs.entries[i]
		}
	}
	panic("spacing: a field of no entry")
}
