package pathfind

import (
	"fmt"

	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
)

// railings says whether a railing keeping d off runs along a cell's band, as a board's read does
type railings interface {
	Railed(c cell.ID, d cell.Domain, deck bool) bool
}

// cellPassages says at which of the board's passages a cell's last changed, as a board's read does
type cellPassages interface {
	CellPassage(c cell.ID) uint64
}

// the most footprints a Finder keeps, the least lately asked for given up for another
const maxPrints = 2 * MaxClasses

// a footprint is where a body bigger than a cell stands: on the nodes every cell of whose window,
// side cells a side, takes it on their level, as of the board's passage it was kept to
type footprint struct {
	domain  cell.Domain
	body    Body // its feet left out
	side    int
	window  []outward // laid from a node
	turned  []outward // the nodes whose windows hold a cell
	stands  []bool    // by node slot: a cell's ordinal twice, plus its level
	passage uint64
	asked   uint64
}

// sideOf is how many cells a side body's footprint takes, unit.Footprint's
func (p *Finder) sideOf(body Body) int {
	if max(body.Size.X, body.Size.Y) <= p.span {
		return 1
	}
	return unit.Footprint(p.grid, body.Size)
}

// extent is how far past the cell it is laid from a footprint side cells a side reaches on an axis
func extent(side int) int { return side / 2 }

// as has p search for body from now on, the search begun: where it is bigger than a cell, with
// its footprint
func (p *Finder) as(body Body) {
	p.body, p.print = body, nil
	if max(body.Size.X, body.Size.Y) > p.span {
		p.print = p.footprintOf(body)
	}
}

// footprintOf is where body, moving in the search's domain, stands as the board's passage has it
// now: nil for a body no bigger than a cell, which stands wherever a cell takes it
func (p *Finder) footprintOf(body Body) *footprint {
	side := p.sideOf(body)
	if side == 1 {
		return nil
	}
	body.Feet = 0
	p.asked++
	spare := -1
	for i, fp := range p.prints {
		if fp.domain == p.domain && fp.body == body {
			fp.asked = p.asked
			p.keep(fp)
			return fp
		}
		if spare < 0 || fp.asked < p.prints[spare].asked {
			spare = i
		}
	}
	var fp *footprint
	if len(p.prints) < maxPrints {
		fp = &footprint{}
		p.prints = append(p.prints, fp)
	} else {
		fp = p.prints[spare]
	}
	fp.domain, fp.body, fp.side, fp.asked = p.domain, body, side, p.asked
	fp.window, fp.turned = p.windowOf(side, false), p.windowOf(side, true)
	p.layPrint(fp)
	return fp
}

// standsOn says whether fp's body stands on c on level
func (p *Finder) standsOn(fp *footprint, c cell.ID, level uint8) bool {
	o, ok := p.grid.Ordinal(c)
	return ok && fp.stands[2*o+int(level)]
}

// layPrint reads anew where fp's body stands, on every node
func (p *Finder) layPrint(fp *footprint) {
	n := 2 * p.grid.CellCount()
	if cap(fp.stands) < n {
		fp.stands = make([]bool, n)
	}
	fp.stands = fp.stands[:n]
	if p.layCell == nil {
		p.layCell = p.restanding
	}
	p.laying = fp
	p.grid.EachCell(p.layCell)
	p.laying = nil
	fp.passage = p.passage()
}

// restanding is restand of the footprint laid, the method value bound once
func (p *Finder) restanding(c cell.ID) { p.restand(p.laying, c) }

// restand reads anew whether fp's body stands on each of c's levels
func (p *Finder) restand(fp *footprint, c cell.ID) {
	o, ok := p.grid.Ordinal(c)
	if !ok {
		return
	}
	fp.stands[2*o+int(ground)] = p.stands(fp, c, ground)
	fp.stands[2*o+int(deck)] = len(p.levelsOf(p.at(c))) > 1 && p.stands(fp, c, deck)
}

// keep has fp stand as the board's passage has it now: the nodes whose windows hold a cell whose
// passage moved read anew, every node where the board says not which or too many moved
func (p *Finder) keep(fp *footprint) {
	now := p.passage()
	if p.passing != nil && fp.passage == now {
		return
	}
	changed, ok := p.changedSince(fp.passage, now, len(fp.window))
	if !ok {
		p.layPrint(fp)
		return
	}
	for _, m := range changed {
		p.ring, p.ringOn = p.rim(m, fp.turned, p.ring, p.ringOn)
		for j, n := range p.ring {
			if p.ringOn[j] {
				p.restand(fp, n)
			}
		}
	}
	fp.passage = now
}

// what the last scan of the cells found: those whose passage moved past since, now the board's
type scanned struct {
	since, now uint64
	done       bool
	cells      []cell.ID
}

// changedSince is the cells whose passage moved past since, now the board's, scanned once for
// every footprint kept to since; false where the board counts no cell's passage, or where fixing
// a footprint round them, ball cells a node, costs more than laying it anew
func (p *Finder) changedSince(since, now uint64, ball int) ([]cell.ID, bool) {
	if p.passing == nil || p.cells == nil {
		return nil, false
	}
	if !p.scan.done || p.scan.since != since || p.scan.now != now {
		p.scan = scanned{since: since, now: now, done: true, cells: p.scan.cells[:0]}
		if p.scanCell == nil {
			p.scanCell = p.noteChange
		}
		p.grid.EachCell(p.scanCell)
	}
	if len(p.scan.cells)*ball > p.grid.CellCount() {
		return nil, false
	}
	return p.scan.cells, true
}

// noteChange notes c where its passage moved past the scan's, the method value bound once
func (p *Finder) noteChange(c cell.ID) {
	if p.cells.CellPassage(c) > p.scan.since {
		p.scan.cells = append(p.scan.cells, c)
	}
}

// stands says whether fp's body stands on c on level: every cell of its window laid from c lies on
// the grid and takes a part of it there
func (p *Finder) stands(fp *footprint, c cell.ID, level uint8) bool {
	var ok bool
	if p.ball, ok = p.round(c, fp.window, p.ball); !ok {
		return false
	}
	var top float32
	if level == deck {
		if p.levels == nil {
			return false
		}
		t, _, carries, raised := p.deckOf(p.at(c))
		if !raised || carries&p.domain == 0 {
			return false
		}
		top = t
	}
	for _, m := range p.ball {
		if !p.takesPart(fp, p.at(m), level, top) {
			return false
		}
	}
	return true
}

// takesPart says whether r's cell takes a part of fp's body on level, the deck it stands on at
// top: on the ground, the ground admitting it and bearing it, no raised deck over it lower than
// it is tall; on a deck, a deck carrying it a step from top at most, admitting and bearing it;
// and no railing along the cell's band on the level
func (p *Finder) takesPart(fp *footprint, r *read, level uint8, top float32) bool {
	if level == deck {
		t, _, carries, raised := p.deckOf(r)
		if !raised || carries&p.domain == 0 || f32.Abs(t-top) > fp.body.Step+reachSlack ||
			!p.stepAt(r, deck).admits || p.limited && !p.borne(r, deck, &fp.body) {
			return false
		}
	} else {
		if !p.groundOf(r).admits || p.limited && !p.borne(r, ground, &fp.body) {
			return false
		}
		if p.levels != nil {
			_, under, _, raised := p.deckOf(r)
			if raised && under-p.levels.Ground(p.grid.CellCenter(r.c)) < fp.body.Height {
				return false
			}
		}
	}
	return !p.railedOn(r, level)
}

// railedOn says whether a railing keeping the search's domain off runs along r's cell's band on
// level, asked once a search
func (p *Finder) railedOn(r *read, level uint8) bool {
	if p.railings == nil {
		return false
	}
	read, on := hasRailing<<level, railing<<level
	if r.has&read == 0 {
		if r.has |= read; p.railings.Railed(r.c, p.domain, level == deck) {
			r.has |= on
		}
	}
	return r.has&on != 0
}

// spans says whether fp's body takes a step onto rn's cell on level by its footprint: it stands
// there, and on a slantwise step's flanks c too, over whose windows its box sweeps. On a hex board
// no step cuts a corner: a box of a footprint three cells a side sweeps past the windows of the
// step's ends by under a hundredth of its side, a wider one's by none.
func (p *Finder) spans(fp *footprint, rn *read, level uint8, c corner) bool {
	return p.standsOn(fp, rn.c, level) &&
		(!c.diagonal || p.standsOn(fp, c.r1.c, level) && p.standsOn(fp, c.r2.c, level))
}

// a cell of a window as the walk out over it reaches it: by the step in direction way from the
// cell from, walked before it; the cell it is laid from's from is -1
type outward struct {
	from int32
	way  int8
}

// windowOf is the walk out over the window of a footprint side cells a side from the cell it is
// laid from, through that cell turned where turned, made the first time it is asked for
func (p *Finder) windowOf(side int, turned bool) []outward {
	k := 2 * side
	if turned {
		k++
	}
	for len(p.windows) <= k {
		p.windows = append(p.windows, nil)
	}
	if p.windows[k] == nil {
		p.windows[k] = windowWalk(p.grid, side, turned)
	}
	return p.windows[k]
}

// windowWalk is the walk out over the window of a footprint side cells a side from the cell it is
// laid from, by their offsets on g's axes, turned through that cell where turned: of an odd side
// the cells (side-1)/2 steps from it or fewer, of an even one, on a grid with corners, the cells
// −(side-1)/2 to side/2 from it on both axes, right and down being after. The cell first, every
// cell a step on from one a step nearer it that never turns back on either axis, so a cell past a
// closed edge of the grid leads on to none on it.
func windowWalk(g grid.Grid, side int, turned bool) []outward {
	type offset struct{ a, b int }
	lo, hi := -(side-1)/2, extent(side)
	if turned {
		lo, hi = -hi, -lo
	}
	if side%2 == 0 && !cornered(g) {
		panic(fmt.Sprintf("pathfind: a footprint of %d cells a side on a grid of no corners", side))
	}
	in := func(o offset) bool {
		return side%2 == 1 || o.a >= lo && o.a <= hi && o.b >= lo && o.b <= hi
	}
	steps := map[offset]int{{}: 0}
	all, ring := []offset{{}}, []offset{{}}
	for k := 1; k <= max(hi, -lo); k++ {
		var next []offset
		for _, o := range ring {
			for i := range g.Directions() {
				da, db := g.Offset(i)
				n := offset{o.a + da, o.b + db}
				if _, seen := steps[n]; !seen {
					steps[n] = k
					next = append(next, n)
				}
			}
		}
		for _, n := range next {
			if in(n) {
				all = append(all, n)
			}
		}
		ring = next
	}
	index := make(map[offset]int32, len(all))
	for j, o := range all {
		index[o] = int32(j)
	}
	walk := make([]outward, len(all))
	walk[0] = outward{from: -1}
	for j := 1; j < len(all); j++ {
		o, found := all[j], false
		for i := range g.Directions() {
			da, db := g.Offset(i)
			from := offset{o.a - da, o.b - db}
			if k, ok := steps[from]; ok && k == steps[o]-1 && in(from) &&
				inward(from.a, o.a) && inward(from.b, o.b) {
				walk[j], found = outward{from: index[from], way: int8(i)}, true
				break
			}
		}
		if !found {
			panic(fmt.Sprintf("pathfind: no walk on a grid's axes out to %v of a footprint of %d "+
				"cells", o, side))
		}
	}
	return walk
}

// cornered says whether a direction of g leaves its cells through a corner, as a square grid's do
func cornered(g grid.Grid) bool {
	for i := range g.Directions() {
		if g.Corner(i) {
			return true
		}
	}
	return false
}

// inward says whether a lies between 0 and b on an axis, b's end taken in
func inward(a, b int) bool { return a == 0 || a*b > 0 && max(a, -a) <= max(b, -b) }

// round is ball holding the cells walk takes out from c, false where one lies off the grid
func (p *Finder) round(c cell.ID, walk []outward, ball []cell.ID) ([]cell.ID, bool) {
	ball = append(ball[:0], c)
	for _, s := range walk[1:] {
		n, ok := p.grid.Toward(ball[s.from], int(s.way))
		if !ok {
			return ball, false
		}
		ball = append(ball, n)
	}
	return ball, true
}

// rim is ring holding the cells walk takes out from c and on whether each lies on the grid: one
// off it leads on to none on it
func (p *Finder) rim(c cell.ID, walk []outward, ring []cell.ID, on []bool) ([]cell.ID, []bool) {
	ring, on = append(ring[:0], c), append(on[:0], true)
	for _, s := range walk[1:] {
		var n cell.ID
		ok := on[s.from]
		if ok {
			n, ok = p.grid.Toward(ring[s.from], int(s.way))
		}
		ring, on = append(ring, n), append(on, ok)
	}
	return ring, on
}
