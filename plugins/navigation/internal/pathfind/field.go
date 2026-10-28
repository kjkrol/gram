package pathfind

import (
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/uid"
)

// Field is the cost of the way to a goal from every node a fill reached, a cell on a level, and
// the step each takes toward it: the routes of a group read off one fill.
type Field struct {
	grid grid.Grid
	goal cell.ID
	cost []float32 // by node: a cell's ordinal twice, plus its level; +Inf unreached
	next []uint8   // by node: the direction of its step, twice, plus the level stepped onto
}

// the next of the goal's own nodes
const fieldEnd = 0xff

// the most cells a board may have for a fill: a group on a larger one searches its routes
const fieldCells = 1 << 18

// how near its own target, by the grid's distance, a walk leaves the field for a search
const fieldLastLeg = 2

// Goal is the cell f leads to.
func (f *Field) Goal() cell.ID { return f.goal }

// Cost is what the way to the goal costs from c on level l, the cheaper of c's levels for
// Either; false where the fill did not reach it.
func (f *Field) Cost(c cell.ID, l Level) (float32, bool) {
	s, ok := f.at(c, l)
	if !ok {
		return 0, false
	}
	return f.cost[s], true
}

// Next is the step from c on level l toward the goal, the cheaper of c's levels for Either: the
// cell stepped into and its level there; false at the goal and where the fill did not reach.
func (f *Field) Next(c cell.ID, l Level) (cell.ID, Level, bool) {
	s, ok := f.at(c, l)
	if !ok || f.next[s] == fieldEnd {
		return 0, Either, false
	}
	n, ok := f.grid.Toward(c, int(f.next[s]>>1))
	if !ok {
		return 0, Either, false
	}
	if f.next[s]&1 == deck {
		return n, Deck, true
	}
	return n, Ground, true
}

// Reached reports whether the fill reached c on either level.
func (f *Field) Reached(c cell.ID) bool {
	_, ok := f.at(c, Either)
	return ok
}

// the node of c on l, the cheaper of the two for Either, where the fill reached it
func (f *Field) at(c cell.ID, l Level) (int, bool) {
	if len(f.cost) == 0 {
		return 0, false
	}
	o, ok := f.grid.Ordinal(c)
	if !ok || 2*o+1 >= len(f.cost) {
		return 0, false
	}
	s := 2 * o
	switch {
	case l == Deck, l == Either && f.cost[s+1] < f.cost[s]:
		s++
	}
	return s, !f32.IsInf(f.cost[s], 1)
}

func (f *Field) node(n node) (int, bool) {
	o, ok := f.grid.Ordinal(n.c)
	if !ok || 2*o+1 >= len(f.cost) || f32.IsInf(f.cost[2*o+int(n.level)], 1) {
		return 0, false
	}
	return 2*o + int(n.level), true
}

// the nodes a fill has yet to settle, the cheapest first: one gone cheaper is pushed again and
// its dearer entry skipped as it comes up; a sift compares as astar's heap does, so ties break
// as there
type frontier struct {
	keys []float32
	ids  []int32
}

func (h *frontier) push(id int32, key float32) {
	h.keys, h.ids = append(h.keys, key), append(h.ids, id)
	j := len(h.keys) - 1
	for j > 0 {
		i := (j - 1) / 2
		if !(key < h.keys[i]) {
			break
		}
		h.keys[j], h.ids[j] = h.keys[i], h.ids[i]
		j = i
	}
	h.keys[j], h.ids[j] = key, id
}

func (h *frontier) pop() (int32, float32, bool) {
	last := len(h.keys) - 1
	if last < 0 {
		return 0, 0, false
	}
	id, top := h.ids[0], h.keys[0]
	key, node := h.keys[last], h.ids[last]
	h.keys, h.ids = h.keys[:last], h.ids[:last]
	if last == 0 {
		return id, top, true
	}
	i := 0
	for {
		j := 2*i + 1
		if j >= last {
			break
		}
		if j2 := j + 1; j2 < last && h.keys[j2] < h.keys[j] {
			j = j2
		}
		if !(h.keys[j] < key) {
			break
		}
		h.keys[i], h.ids[i] = h.keys[j], h.ids[j]
		i = j
	}
	h.keys[i], h.ids[i] = key, node
	return id, top, true
}

// FillField fills f with the way to goal, ending on its level end, for a body of body's class
// moving in domain: nodes settle from the goal till each seed's cell is reached and the front
// costs more than margin times the farthest, every step priced as Find prices it. It reports how
// many seeds it reached; a Finder without levels or a board past fieldCells cells fills none.
func (p *Finder) FillField(f *Field, goal cell.ID, end Level, domain cell.Domain, body Body,
	seeds []cell.ID, margin float32) int {
	f.grid, f.goal = p.grid, goal
	count := p.grid.CellCount()
	o, on := p.grid.Ordinal(goal)
	if p.levels == nil || count > fieldCells || !on {
		f.cost, f.next = f.cost[:0], f.next[:0]
		return 0
	}
	p.fills++
	p.body, p.entity, p.blocked = body, 0, nil
	p.begin(domain)
	if cap(f.cost) < 2*count {
		f.cost, f.next = make([]float32, 2*count), make([]uint8, 2*count)
	}
	f.cost, f.next = f.cost[:2*count], f.next[:2*count]
	for i := range f.cost {
		f.cost[i] = f32.Inf(1)
	}
	want := p.stampSeeds(seeds)
	h := &p.frontier
	h.keys, h.ids = h.keys[:0], h.ids[:0]
	levels := p.levelsOf(p.at(goal))
	if len(levels) == 1 {
		end = Either
	}
	for _, l := range levels {
		if end == Either || end == Deck == (l == deck) {
			f.cost[2*o+int(l)], f.next[2*o+int(l)] = 0, fieldEnd
			h.push(int32(2*o+int(l)), 0)
		}
	}
	reached, farthest := 0, float32(0)
	for {
		s, key, ok := h.pop()
		if !ok {
			break
		}
		if key > f.cost[s] {
			continue
		}
		if reached == want && key > farthest*margin {
			f.cost[s] = f32.Inf(1)
			break
		}
		// every node pushed had its cell read this fill
		at := int(s) >> 1
		if p.seeded[at] == p.seeding {
			p.seeded[at] = 0
			reached, farthest = reached+1, key
		}
		p.into(f, node{p.reads[at].c, uint8(s & 1)}, key)
	}
	// what was left on the front is a cost not yet the least: unreached
	for i, s := range h.ids {
		if h.keys[i] == f.cost[s] {
			f.cost[s] = f32.Inf(1)
		}
	}
	return reached
}

// stampSeeds marks the cells of seeds, each once, for this fill and reports how many
func (p *Finder) stampSeeds(seeds []cell.ID) int {
	if len(p.seeded) != p.grid.CellCount() {
		p.seeded = make([]uint32, p.grid.CellCount())
	}
	if p.seeding++; p.seeding == 0 {
		clear(p.seeded)
		p.seeding = 1
	}
	n := 0
	for _, c := range seeds {
		if o, ok := p.grid.Ordinal(c); ok && p.seeded[o] != p.seeding {
			p.seeded[o] = p.seeding
			n++
		}
	}
	return n
}

// into has each node a step from which reaches t, settled at cost, go there if that is its
// cheaper way: the step checked and priced as leveledFrom checks it from that node
func (p *Finder) into(f *Field, t node, cost float32) {
	rt := p.at(t.c)
	for i := range p.grid.Directions() {
		n, ok := p.grid.Toward(t.c, i)
		if !ok {
			continue
		}
		o, ok := p.grid.Ordinal(n)
		if !ok {
			continue
		}
		rn := p.at(n)
		back := p.grid.Opposite(i)
		c := p.cornerOf(n, t.c, back)
		cut := c.diagonal && p.cutOff(c)
		for _, level := range p.levelsOf(rn) {
			price, ok := p.step(node{n, level}, rn, rt, t.level, c, cut)
			if !ok {
				continue
			}
			s := 2*o + int(level)
			if v := float32(float64(cost) + price); v < f.cost[s] {
				f.cost[s], f.next[s] = v, uint8(back)<<1|t.level
				p.frontier.push(int32(s), v)
			}
		}
	}
}

// Walk writes the route from from to target, ending on its level end, for entity moving in
// domain into dst as Find writes it, read off f, filled for body's class: down the field's
// steps, and to a target other than f's goal the last leg searched once the walk comes within
// fieldLastLeg of it or to the goal. False where f did not reach from or that leg found nothing.
func (p *Finder) Walk(f *Field, entity uid.UID64, domain cell.Domain, body Body,
	from, target cell.ID, end Level, dst []cell.ID) (int, bool) {
	p.walks++
	if p.levels == nil || len(f.cost) == 0 {
		return 0, false
	}
	p.decks, p.body, p.entity = 0, body, entity
	p.begin(domain)
	at := node{from, p.levelOf(p.at(from), body)}
	s, ok := f.node(at)
	if !ok {
		return 0, false
	}
	var decks uint64
	n := 0
	for n < len(dst) {
		if target != f.goal && (at.c == f.goal || p.grid.Distance(at.c, target) <= fieldLastLeg) {
			p.searches++
			k, ok := p.findFrom(at, target, end, dst[n:])
			if !ok {
				return 0, false
			}
			p.decks = decks | p.decks<<n
			return n + k, true
		}
		w := f.next[s]
		if w == fieldEnd {
			break
		}
		c, _ := p.grid.Toward(at.c, int(w>>1))
		at = node{c, w & 1}
		s, _ = f.node(at)
		dst[n] = c
		if at.level == deck && n < 64 {
			decks |= 1 << n
		}
		n++
	}
	p.decks, p.ended = decks, at.level
	return n, n > 0
}

// Fills counts the fields filled since the Finder was made.
func (p *Finder) Fills() uint64 { return p.fills }

// Walks counts the routes walked off fields since the Finder was made, those that found none too.
func (p *Finder) Walks() uint64 { return p.walks }
